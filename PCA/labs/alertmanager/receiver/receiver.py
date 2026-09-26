"""Receiver de webhook "espião" para o lab de Alertmanager.

O Alertmanager manda cada notificação para POST /hook/<nome> (ex.: /hook/pager,
/hook/slack). Aqui nada é enviado de verdade: o payload é guardado e impresso,
para você ver EXATAMENTE o que chegou, quando, em qual receiver e agrupado como.

  POST /hook/<nome>          endpoint de webhook (use no alertmanager.yml)
  GET  /received             todas as notificações (JSON, mais antigas primeiro)
  GET  /received?hook=pager  só as que chegaram em /hook/pager
  GET  /received?raw=1       inclui o payload original completo
  POST /reset  (ou GET)      apaga o histórico
"""
import json
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

received = []
lock = threading.Lock()


def summarize(hook, p):
    return {
        "n": 0,
        "at": time.strftime("%H:%M:%S"),
        "hook": hook,
        "receiver": p.get("receiver"),
        "status": p.get("status"),
        "groupKey": p.get("groupKey"),
        "groupLabels": p.get("groupLabels", {}),
        "commonLabels": p.get("commonLabels", {}),
        "truncatedAlerts": p.get("truncatedAlerts", 0),
        "alerts": [
            {
                "status": a.get("status"),
                "labels": a.get("labels", {}),
                "annotations": a.get("annotations", {}),
                "startsAt": a.get("startsAt"),
                "endsAt": a.get("endsAt"),
            }
            for a in p.get("alerts", [])
        ],
    }


class H(BaseHTTPRequestHandler):
    def reply(self, code, body, ctype="application/json"):
        data = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def reset(self):
        with lock:
            received.clear()
        print("[receiver] histórico apagado", flush=True)
        return self.reply(200, '{"ok": true}\n')

    def do_POST(self):
        u = urlparse(self.path)
        if u.path == "/reset":
            return self.reset()
        if not u.path.startswith("/hook/"):
            return self.reply(404, '{"error": "use /hook/<nome>"}\n')
        hook = u.path[len("/hook/"):] or "default"
        body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
        try:
            payload = json.loads(body)
        except ValueError:
            return self.reply(400, '{"error": "json inválido"}\n')
        s = summarize(hook, payload)
        with lock:
            s["n"] = len(received) + 1
            received.append({**s, "raw": payload})
        names = ", ".join(
            f"{a['labels'].get('alertname')}{{{','.join(f'{k}={v}' for k, v in a['labels'].items() if k != 'alertname')}}}[{a['status']}]"
            for a in s["alerts"]
        )
        print(f"[receiver] #{s['n']} {s['at']} hook={hook} status={s['status']} "
              f"group={json.dumps(s['groupLabels'], sort_keys=True)} alerts={len(s['alerts'])}: {names}", flush=True)
        return self.reply(200, '{"ok": true}\n')

    def do_GET(self):
        u = urlparse(self.path)
        q = parse_qs(u.query)
        if u.path == "/reset":
            return self.reset()
        if u.path == "/received":
            hook = q.get("hook", [None])[0]
            raw = q.get("raw", ["0"])[0] == "1"
            with lock:
                out = [r if raw else {k: v for k, v in r.items() if k != "raw"}
                       for r in received if hook is None or r["hook"] == hook]
            return self.reply(200, json.dumps(out, indent=2, ensure_ascii=False) + "\n")
        if u.path in ("/", "/-/healthy"):
            return self.reply(200, __doc__, "text/plain; charset=utf-8")
        return self.reply(404, '{"error": "404"}\n')

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    print("[receiver] ouvindo em :8080", flush=True)
    ThreadingHTTPServer(("0.0.0.0", 8080), H).serve_forever()
