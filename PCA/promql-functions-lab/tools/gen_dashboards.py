#!/usr/bin/env python3
"""Gera um dashboard Grafana por função a partir de functions/<fn>/lab.yaml.

Formato do lab.yaml (veja functions/rate/lab.yaml como exemplo completo):

function: rate                 # nome da função (= nome da pasta)
title: "rate() — velocidade média de um contador"
category: "02 - Contadores"    # vira pasta no Grafana
time_from: now-15m             # opcional
intro: |                       # markdown do painel de topo
  ...
panels:
  - title: "..."
    description: "..."        # markdown (aparece no (i) do painel)
    type: timeseries           # timeseries | stat | table | bargauge | gauge | heatmap | text
    options: {}                # opcional: merge bruto em panel.options do Grafana
    field_defaults: {}         # opcional: merge bruto em fieldConfig.defaults
    overrides: []              # opcional: fieldConfig.overrides do Grafana
    hide_columns: [Time, env, job]  # opcional (table): colunas escondidas
    transformations: []        # opcional: substitui as transformations do painel
    width: 12                  # 1..24 (default 12)
    height: 9                  # default 9
    unit: reqps                # opcional (unidade do Grafana)
    decimals: 2                # opcional
    min: 0                     # opcional
    max: 100                   # opcional
    stack: false               # opcional (timeseries)
    draw: line                 # line | points | bars (timeseries)
    content: "..."             # só p/ type: text
    queries:
      - expr: 'rate(x[1m])'
        legend: "{{job}}"
        instant: false         # true -> consulta instantânea (stat/table/bargauge)
        validate: nonempty     # nonempty | empty | any  (usado por tools/validate.py)
        format: heatmap        # opcional: time_series | table | heatmap
"""
import json
import pathlib
import re
import sys

import yaml

ROOT = pathlib.Path(__file__).resolve().parent.parent
FUNCS = ROOT / "functions"
OUT = ROOT / "dashboards"

DS = {"type": "prometheus", "uid": "prometheus"}


def slug(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")


def target(q, i, ptype):
    instant = q.get("instant", ptype in ("stat", "table", "bargauge", "gauge"))
    t = {
        "refId": chr(ord("A") + i),
        "datasource": DS,
        "expr": q["expr"].strip(),
        "legendFormat": q.get("legend", "__auto"),
        "range": not instant,
        "instant": instant,
        "editorMode": "code",
    }
    if ptype == "table":
        t["format"] = "table"
    if ptype == "heatmap" and q.get("format", "heatmap") == "heatmap":
        t["format"] = "heatmap"
    if "format" in q:
        t["format"] = q["format"]
    return t


def panel(p, pid, x, y):
    ptype = p.get("type", "timeseries")
    w, h = p.get("width", 12), p.get("height", 9)
    base = {
        "id": pid,
        "type": ptype,
        "title": p["title"],
        "description": p.get("description", ""),
        "gridPos": {"x": x, "y": y, "w": w, "h": h},
    }
    if ptype == "text":
        # "~" em pares vira riscado no markdown do Grafana; escapa fora de blocos de código
        parts = re.split(r"(```.*?```|`[^`]*`)", p.get("content", ""), flags=re.S)
        content = "".join(x if x.startswith("`") else re.sub(r"(?<!\\)~", r"\\~", x) for x in parts)
        base["options"] = {"mode": "markdown", "content": content}
        return base
    defaults = {"color": {"mode": "palette-classic"}, "custom": {}}
    for k in ("unit", "decimals", "min", "max"):
        if k in p:
            defaults[k] = p[k]
    if ptype == "timeseries":
        draw = p.get("draw", "line")
        defaults["custom"] = {
            "drawStyle": draw,
            "lineWidth": 2,
            "fillOpacity": 10 if draw == "line" else 60,
            "pointSize": 5,
            "showPoints": "auto" if draw != "points" else "always",
            "spanNulls": False,
            "stacking": {"mode": "normal" if p.get("stack") else "none"},
        }
        base["options"] = {
            "legend": {"displayMode": "table", "placement": "bottom", "calcs": ["lastNotNull", "min", "max"]},
            "tooltip": {"mode": "multi", "sort": "desc"},
        }
    elif ptype == "stat":
        defaults["color"] = {"mode": "thresholds"}
        defaults["thresholds"] = {"mode": "absolute", "steps": [{"color": "blue", "value": None}]}
        # NaN/±Inf aparecem como texto (e não como painel em branco) — importante p/ ensinar casos especiais
        defaults["noValue"] = "vazio (no data)"
        defaults["mappings"] = [{"type": "special", "options": {"match": "nan", "result": {"text": "NaN", "color": "red", "index": 0}}}]
        base["options"] = {
            "reduceOptions": {"calcs": ["last"], "fields": "", "values": False},
            "textMode": "value_and_name", "colorMode": "background", "graphMode": "none",
            "justifyMode": "auto", "orientation": "auto",
        }
    elif ptype in ("bargauge", "gauge"):
        defaults["color"] = {"mode": "continuous-BlYlRd"}
        base["options"] = {
            "reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
            "displayMode": "gradient", "orientation": "horizontal", "showUnfilled": True,
        }
    elif ptype == "table":
        base["options"] = {"showHeader": True, "cellHeight": "sm"}
        # esconde colunas-ruído (Time, env, job...) p/ o Value caber na tela;
        # override por painel: hide_columns: [...] ou transformations: [...]
        hide = p.get("hide_columns", ["Time", "env", "job"])
        base["transformations"] = [
            {"id": "merge", "options": {}},
            {"id": "organize", "options": {"excludeByName": {c: True for c in hide}}},
        ]
    elif ptype == "heatmap":
        base["options"] = {
            "calculate": False, "cellGap": 1, "yAxis": {"axisPlacement": "left"},
            "color": {"mode": "scheme", "scheme": "Spectral", "steps": 64},
            "rowsFrame": {"layout": "auto"}, "tooltip": {"mode": "single", "yHistogram": True},
        }
    if "transformations" in p:
        base["transformations"] = p["transformations"]
    # passthrough bruto para casos especiais
    defaults.update(p.get("field_defaults", {}))
    if "options" in p:
        base.setdefault("options", {}).update(p["options"])
    base["fieldConfig"] = {"defaults": defaults, "overrides": p.get("overrides", [])}
    base["datasource"] = DS
    base["targets"] = [target(q, i, ptype) for i, q in enumerate(p.get("queries", []))]
    return base


def build(lab, readme_url):
    fn = lab["function"]
    panels, pid, x, y = [], 1, 0, 0
    intro = lab.get("intro", "").rstrip() + f"\n\n📘 Guia completo: `functions/{fn}/README.md`"
    ih = lab.get("intro_height", 6)
    panels.append(panel({"type": "text", "title": lab["title"], "content": intro, "width": 24, "height": ih}, pid, 0, 0))
    y = ih
    row_h = 0
    for p in lab.get("panels", []):
        pid += 1
        w = p.get("width", 12)
        if x + w > 24:
            x, y = 0, y + row_h
            row_h = 0
        panels.append(panel(p, pid, x, y))
        x += w
        row_h = max(row_h, p.get("height", 9))
    return {
        "uid": f"fn-{fn}"[:40],
        "title": lab["title"],
        "tags": ["promql-lab", slug(lab.get("category", "misc"))],
        "timezone": "browser",
        "editable": True,
        "graphTooltip": 1,
        "refresh": lab.get("refresh", "10s"),
        "time": {"from": lab.get("time_from", "now-15m"), "to": "now"},
        "schemaVersion": 41,
        "panels": panels,
        "links": [{"title": "Todas as funções", "type": "dashboards", "tags": ["promql-lab"], "asDropdown": True}],
    }


def main():
    only = set(sys.argv[1:])
    # limpa dashboards antigos gerados
    for old in OUT.glob("**/*.json"):
        if not only:
            old.unlink()
    n, errors = 0, 0
    for labf in sorted(FUNCS.glob("*/lab.yaml")):
        fn = labf.parent.name
        if only and fn not in only:
            continue
        try:
            lab = yaml.safe_load(labf.read_text())
            assert lab.get("function") == fn, f"function={lab.get('function')} != pasta {fn}"
            dash = build(lab, None)
        except Exception as e:  # noqa
            print(f"ERRO {labf}: {e}", file=sys.stderr)
            errors += 1
            continue
        cat = lab.get("category", "99 - Outros")
        d = OUT / cat
        d.mkdir(parents=True, exist_ok=True)
        (d / f"{fn}.json").write_text(json.dumps(dash, indent=2, ensure_ascii=False))
        n += 1
    print(f"{n} dashboards gerados, {errors} erros")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
