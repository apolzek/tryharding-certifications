#!/usr/bin/env python3
"""Gera dashboards/slo.json (rode de novo se mudar algo: python3 tools/build-dashboard.py)."""
import json
import pathlib

DS = {"type": "prometheus", "uid": "prometheus"}
_id = 0


def nid():
    global _id
    _id += 1
    return _id


def target(expr, legend="", ref="A", instant=False, fmt="time_series"):
    t = {"datasource": DS, "expr": expr, "legendFormat": legend or "__auto", "refId": ref, "format": fmt}
    if instant:
        t.update({"instant": True, "range": False})
    return t


def stat(title, expr, x, y, w=4, h=5, unit="percentunit", decimals=2, steps=None, desc=""):
    return {
        "id": nid(), "type": "stat", "title": title, "description": desc, "datasource": DS,
        "gridPos": {"x": x, "y": y, "w": w, "h": h},
        "targets": [target(expr)],
        "fieldConfig": {"defaults": {
            "unit": unit, "decimals": decimals, "color": {"mode": "thresholds"},
            "thresholds": {"mode": "absolute", "steps": steps or [{"color": "green", "value": None}]},
        }, "overrides": []},
        "options": {"reduceOptions": {"calcs": ["lastNotNull"], "fields": "", "values": False},
                    "colorMode": "background", "graphMode": "area", "textMode": "value",
                    "justifyMode": "auto", "orientation": "auto", "wideLayout": True},
    }


def ts(title, targets, x, y, w=12, h=9, unit="short", decimals=None, log=False, thresholds=None, desc="", minv=None, maxv=None):
    custom = {"drawStyle": "line", "lineWidth": 2, "fillOpacity": 5, "showPoints": "never",
              "spanNulls": True, "thresholdsStyle": {"mode": "dashed" if thresholds else "off"}}
    if log:
        custom["scaleDistribution"] = {"type": "log", "log": 10}
    d = {"unit": unit, "custom": custom, "color": {"mode": "palette-classic"},
         "thresholds": {"mode": "absolute", "steps": thresholds or [{"color": "green", "value": None}]}}
    if decimals is not None:
        d["decimals"] = decimals
    if minv is not None:
        d["min"] = minv
    if maxv is not None:
        d["max"] = maxv
    return {"id": nid(), "type": "timeseries", "title": title, "description": desc, "datasource": DS,
            "gridPos": {"x": x, "y": y, "w": w, "h": h}, "targets": targets,
            "fieldConfig": {"defaults": d, "overrides": []},
            "options": {"legend": {"displayMode": "list", "placement": "bottom", "showLegend": True},
                        "tooltip": {"mode": "multi", "sort": "desc"}}}


def row(title, y):
    return {"id": nid(), "type": "row", "title": title, "collapsed": False, "panels": [],
            "gridPos": {"x": 0, "y": y, "w": 24, "h": 1}}


G, Y, O, R = "green", "yellow", "orange", "red"
panels = [
    row("Agora (escala 1:60: 1 min do lab = 1 h de produção)", 0),
    stat("Disponibilidade · 1m (≈1h)", "1 - job:slo_errors_per_request:ratio_rate1m", 0, 1, decimals=3,
         steps=[{"color": R, "value": None}, {"color": G, "value": 0.999}],
         desc="SLI de disponibilidade: fração de requisições sem 5xx. SLO = 99,9%."),
    stat("Latência < 300ms · 1m (≈1h)", "1 - job:slo_latency_slow_per_request:ratio_rate1m", 4, 1, decimals=2,
         steps=[{"color": R, "value": None}, {"color": G, "value": 0.99}],
         desc="SLI de latência: fração de requisições em até 300ms (bucket le=0.3 / count). SLO = 99%."),
    stat("Error budget restante · 12h (≈30d)", "job:slo_error_budget_remaining:ratio", 8, 1, decimals=1,
         steps=[{"color": R, "value": None}, {"color": O, "value": 0}, {"color": Y, "value": 0.25}, {"color": G, "value": 0.5}],
         desc="1 - (razão de erro na janela do SLO / 0,001). Negativo = SLO violado. Com a stack recém-criada a janela de 12h tem só minutos de dados, então qualquer incidente pesa muito."),
    stat("Burn rate · 1m (≈1h)", "job:slo_errors_per_request:ratio_rate1m / 0.001", 12, 1, unit="none", decimals=1,
         steps=[{"color": G, "value": None}, {"color": Y, "value": 1}, {"color": O, "value": 6}, {"color": R, "value": 14.4}],
         desc="Quantas vezes mais rápido que o sustentável o budget está sendo gasto. 1 = gasta tudo exatamente no fim da janela."),
    stat("Chaos: error_rate", "chaos_error_rate", 16, 1, decimals=2,
         steps=[{"color": G, "value": None}, {"color": R, "value": 0.001}]),
    stat("Chaos: latência extra", "chaos_latency_seconds", 20, 1, unit="s", decimals=2,
         steps=[{"color": G, "value": None}, {"color": R, "value": 0.001}]),

    row("Burn rate multi-janela e alertas", 6),
    ts("Burn rate por janela (escala log)", [
        target("job:slo_errors_per_request:ratio_rate5s / 0.001", "5s (≈5m)", "A"),
        target("job:slo_errors_per_request:ratio_rate30s / 0.001", "30s (≈30m)", "B"),
        target("job:slo_errors_per_request:ratio_rate1m / 0.001", "1m (≈1h)", "C"),
        target("job:slo_errors_per_request:ratio_rate6m / 0.001", "6m (≈6h)", "D"),
        target("job:slo_errors_per_request:ratio_rate24m / 0.001", "24m (≈1d)", "E"),
        target("job:slo_errors_per_request:ratio_rate72m / 0.001", "72m (≈3d)", "F"),
    ], 0, 7, w=14, h=10, unit="none", log=True, decimals=1,
        thresholds=[{"color": "transparent", "value": None}, {"color": Y, "value": 1}, {"color": O, "value": 6},
                    {"color": R, "value": 14.4}],
        desc="Linhas de limiar: 1x, 6x e 14.4x. O fast burn exige 1m E 5s acima de 14.4x."),
    {
        "id": nid(), "type": "state-timeline", "title": "Estado dos alertas de SLO", "datasource": DS,
        "gridPos": {"x": 14, "y": 7, "w": 10, "h": 10},
        "targets": [target('max by (alertname) (ALERTS{alertstate="firing",slo!=""}) * 2 '
                           'or max by (alertname) (ALERTS{alertstate="pending",slo!=""})', "{{alertname}}")],
        "fieldConfig": {"defaults": {
            "color": {"mode": "fixed", "fixedColor": "text"},
            "thresholds": {"mode": "absolute", "steps": [{"color": "text", "value": None}]},
            "mappings": [
                {"type": "range", "options": {"from": 0.5, "to": 1.5, "result": {"text": "pending", "color": Y, "index": 0}}},
                {"type": "range", "options": {"from": 1.5, "to": 2.5, "result": {"text": "firing", "color": R, "index": 1}}},
            ],
        }, "overrides": []},
        "options": {"showValue": "auto", "mergeValues": True, "rowHeight": 0.8, "alignValue": "center",
                    "legend": {"showLegend": False}},
    },

    row("SLIs e budget ao longo do tempo", 17),
    ts("SLI de disponibilidade", [
        target("1 - job:slo_errors_per_request:ratio_rate5s", "5s (≈5m)", "A"),
        target("1 - job:slo_errors_per_request:ratio_rate1m", "1m (≈1h)", "B"),
    ], 0, 18, w=8, h=8, unit="percentunit", decimals=2, maxv=1,
        thresholds=[{"color": R, "value": None}, {"color": "transparent", "value": 0.999}]),
    ts("SLI de latência (< 300ms)", [
        target("1 - job:slo_latency_slow_per_request:ratio_rate5s", "5s (≈5m)", "A"),
        target("1 - job:slo_latency_slow_per_request:ratio_rate1m", "1m (≈1h)", "B"),
        target('histogram_fraction(0, 0.3, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[1m])))',
               "histogram_fraction 1m", "C"),
    ], 8, 18, w=8, h=8, unit="percentunit", decimals=2, maxv=1,
        thresholds=[{"color": R, "value": None}, {"color": "transparent", "value": 0.99}]),
    ts("Error budget restante (janela 12h ≈ 30d)", [
        target("job:slo_error_budget_remaining:ratio", "budget restante", "A"),
    ], 16, 18, w=8, h=8, unit="percentunit", decimals=1,
        thresholds=[{"color": R, "value": None}, {"color": "transparent", "value": 0}]),
    ts("Tráfego por status", [
        target('sum by (code) (rate(http_requests_total{job="checkout"}[5s]))', "{{code}}", "A"),
    ], 0, 26, w=12, h=7, unit="reqps"),
    ts("Latência p50 / p99", [
        target('histogram_quantile(0.5, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[30s])))', "p50", "A"),
        target('histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[30s])))', "p99", "B"),
    ], 12, 26, w=12, h=7, unit="s",
        thresholds=[{"color": "transparent", "value": None}, {"color": R, "value": 0.3}]),
]

dash = {
    "uid": "slo-checkout", "title": "SLO · checkout (lab 1:60)", "tags": ["pca", "slo"],
    "timezone": "browser", "schemaVersion": 41, "version": 1, "editable": True,
    "refresh": "5s", "time": {"from": "now-15m", "to": "now"},
    "annotations": {"list": []}, "templating": {"list": []}, "panels": panels,
}
out = pathlib.Path(__file__).resolve().parent.parent / "dashboards" / "slo.json"
out.write_text(json.dumps(dash, indent=2, ensure_ascii=False) + "\n")
print(f"ok: {out}")
