#!/usr/bin/env python3
"""Valida uma (ou mais) lições e, opcionalmente, notifica o Discord.

  python3 tools/validate.py rate increase            # só valida
  python3 tools/validate.py rate --notify            # valida + print + Discord
  python3 tools/validate.py rate --notify --force    # re-notifica mesmo se já notificado

Checks:
  1. arquivos: README.md, lab.yaml, setup/scenario.go
  2. dashboard provisionado no Grafana (uid fn-<fn>)
  3. cada query do lab.yaml roda no Prometheus sem erro e o resultado bate
     com `validate:` (nonempty [default] | empty | any)
  4. screenshot do dashboard (chromium headless) -> .lab-status/<fn>.png
"""
import argparse
import json
import os
import pathlib
import re
import subprocess
import sys
import time

import requests
import yaml

ROOT = pathlib.Path(__file__).resolve().parent.parent
STATUS = ROOT / ".lab-status"
STATUS.mkdir(exist_ok=True)
PROM = os.environ.get("PROM_URL", "http://localhost:9095")
GRAFANA = os.environ.get("GRAFANA_URL", "http://localhost:3300")
WEBHOOK = os.environ.get("DISCORD_WEBHOOK", "")
WEBHOOK_FILE = ROOT / ".discord-webhook"


def webhook():
    if WEBHOOK:
        return WEBHOOK
    if WEBHOOK_FILE.exists():
        return WEBHOOK_FILE.read_text().strip()
    return ""


def query(expr, instant):
    try:
        if instant:
            r = requests.post(f"{PROM}/api/v1/query", data={"query": expr}, timeout=30)
        else:
            end = time.time()
            r = requests.post(f"{PROM}/api/v1/query_range",
                              data={"query": expr, "start": end - 600, "end": end, "step": 15}, timeout=30)
        j = r.json()
    except Exception as e:  # noqa
        return False, f"erro HTTP: {e}", 0
    if j.get("status") != "success":
        return False, f"{j.get('errorType')}: {j.get('error')}", 0
    res = j["data"]["result"]
    if j["data"]["resultType"] == "scalar":
        return True, "scalar", 1
    return True, "", len(res)


def screenshot(fn, uid, out):
    url = f"{GRAFANA}/d/{uid}?orgId=1&kiosk&refresh=&from=now-15m&to=now-30s"
    # altura calculada a partir do grid do dashboard (1 unidade ≈ 38px) p/ não cortar painéis
    height = 1500
    try:
        dash = requests.get(f"{GRAFANA}/api/dashboards/uid/{uid}", timeout=10).json()["dashboard"]
        bottom = max(p["gridPos"]["y"] + p["gridPos"]["h"] for p in dash["panels"])
        height = max(900, min(6000, bottom * 38 + 120))
    except Exception:  # noqa
        pass
    cmd = ["chromium", "--headless=new", "--no-sandbox", "--disable-gpu", "--hide-scrollbars",
           f"--window-size=1600,{height}", "--virtual-time-budget=20000", f"--screenshot={out}", url]
    subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=90)
    return out.exists()


def notify(msg, files):
    hook = webhook()
    if not hook:
        print("  (sem webhook configurado — pulei notificação)")
        return
    data = {"payload_json": json.dumps({"content": msg[:1990]})}
    fh = {f"files[{i}]": (p.name, open(p, "rb"), "image/png") for i, p in enumerate(files)}
    r = requests.post(hook, data=data, files=fh, timeout=60)
    print(f"  discord: HTTP {r.status_code}")


def validate(fn, do_notify, force):
    d = ROOT / "functions" / fn
    problems, lines = [], []
    for f in ("README.md", "lab.yaml", "setup/scenario.go"):
        if not (d / f).exists():
            problems.append(f"falta {f}")
    if problems:
        return False, problems, lines, None
    lab = yaml.safe_load((d / "lab.yaml").read_text())
    uid = f"fn-{fn}"[:40]
    try:
        for _ in range(6):  # Grafana leva alguns segundos p/ recarregar dashboards provisionados
            r = requests.get(f"{GRAFANA}/api/dashboards/uid/{uid}", timeout=10)
            if r.status_code == 200:
                break
            time.sleep(5)
        if r.status_code != 200:
            problems.append(f"dashboard {uid} não encontrado no Grafana (HTTP {r.status_code})")
    except Exception as e:  # noqa
        problems.append(f"grafana indisponível: {e}")
    nq = 0
    for p in lab.get("panels", []):
        ptype = p.get("type", "timeseries")
        for q in p.get("queries", []):
            nq += 1
            exp = q.get("validate", "nonempty")
            instant = q.get("instant", ptype in ("stat", "table", "bargauge", "gauge"))
            ok, err, n = query(q["expr"], instant)
            short = " ".join(q["expr"].split())[:90]
            if not ok:
                problems.append(f"query falhou: `{short}` -> {err}")
            elif exp == "nonempty" and n == 0:
                problems.append(f"resultado vazio (esperado dados): `{short}`")
            elif exp == "empty" and n > 0:
                problems.append(f"esperado vazio mas veio {n} séries: `{short}`")
            else:
                lines.append(f"✔ `{short}` → {n} série(s)" if n or exp != "empty" else f"✔ `{short}` → vazio (esperado)")
    shot = STATUS / f"{fn}.png"
    if shot.exists():
        shot.unlink()
    if not problems:
        screenshot(fn, uid, shot)
    return not problems, problems, lines, (shot if shot.exists() else None), lab, nq


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("functions", nargs="+")
    ap.add_argument("--notify", action="store_true")
    ap.add_argument("--force", action="store_true")
    a = ap.parse_args()
    rc = 0
    for fn in a.functions:
        res = validate(fn, a.notify, a.force)
        ok, problems, lines = res[0], res[1], res[2]
        shot = res[3]
        print(f"== {fn}: {'OK' if ok else 'FALHOU'}")
        for l in lines:
            print("  ", l)
        for pr in problems:
            print("   ✘", pr)
        if not ok:
            rc = 1
            continue
        lab, nq = res[4], res[5]
        stf = STATUS / f"{fn}.json"
        already = stf.exists() and json.loads(stf.read_text()).get("notified")
        if a.notify and (force := a.force or not already):
            msg = (f"✅ **`{fn}()`** validada — {lab['title']}\n"
                   f"📁 `functions/{fn}/`  •  {nq} queries testadas no Prometheus v3.15.0  •  "
                   f"dashboard Grafana 13.2.2 `fn-{fn}`\n" + "\n".join(lines[:8]))
            notify(msg, [shot] if shot else [])
            stf.write_text(json.dumps({"notified": True, "ts": time.time()}))
        elif a.notify:
            print("  (já notificado — use --force)")
    return rc


if __name__ == "__main__":
    sys.exit(main())
