"""Fonte de métricas "de brinquedo" para o lab de Alertmanager.

Você liga/desliga gauges por HTTP e o Prometheus raspa /metrics. Assim cada
alerta dispara exatamente quando você quer (exercícios determinísticos).

  GET /set?name=lab_up&value=0&instance=api-1&cluster=eu   cria/atualiza uma série
  GET /del?name=lab_up&instance=api-1&cluster=eu           remove uma série
  GET /reset                                               remove todas
  GET /list                                                séries atuais (JSON)
  GET /metrics                                             formato de exposição

Todo parâmetro que não é `name`/`value` vira label.
"""
import json
import re
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qsl, urlparse

NAME_RE = re.compile(r"^[a-zA-Z_:][a-zA-Z0-9_:]*$")
LABEL_RE = re.compile(r"^[a-zA-Z_][a-zA-Z0-9_]*$")
series = {}  # (name, ((k, v), ...)) -> float
lock = threading.Lock()


def esc(v):
    return v.replace("\\", "\\\\").replace("\n", "\\n").replace('"', '\\"')


class H(BaseHTTPRequestHandler):
    def reply(self, code, body, ctype="text/plain; charset=utf-8"):
        data = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def key(self, q):
        name = q.pop("name", "")
        if not NAME_RE.match(name):
            raise ValueError(f"nome de métrica inválido: {name!r}")
        q.pop("value", None)
        for k in q:
            if not LABEL_RE.match(k) or k.startswith("__"):
                raise ValueError(f"label inválida: {k!r}")
        return (name, tuple(sorted(q.items())))

    def do_GET(self):
        u = urlparse(self.path)
        q = dict(parse_qsl(u.query, keep_blank_values=True))
        try:
            if u.path == "/metrics":
                with lock:
                    items = sorted(series.items())
                lines, typed = [], set()
                for (name, labels), val in items:
                    if name not in typed:
                        lines.append(f"# TYPE {name} gauge")
                        typed.add(name)
                    lbl = ",".join(f'{k}="{esc(v)}"' for k, v in labels)
                    lines.append(f"{name}{{{lbl}}} {val}" if lbl else f"{name} {val}")
                return self.reply(200, "\n".join(lines) + "\n", "text/plain; version=0.0.4")
            if u.path == "/set":
                value = float(q.get("value", "1"))
                k = self.key(q)
                with lock:
                    series[k] = value
                return self.reply(200, f"ok {k[0]}{dict(k[1])} = {value}\n")
            if u.path == "/del":
                k = self.key(q)
                with lock:
                    existed = series.pop(k, None) is not None
                return self.reply(200, f"{'removida' if existed else 'não existia'} {k[0]}{dict(k[1])}\n")
            if u.path == "/reset":
                with lock:
                    series.clear()
                return self.reply(200, "ok, todas as séries removidas\n")
            if u.path == "/list":
                with lock:
                    out = [{"name": n, "labels": dict(l), "value": v} for (n, l), v in sorted(series.items())]
                return self.reply(200, json.dumps(out, indent=2) + "\n", "application/json")
            if u.path in ("/", "/-/healthy"):
                return self.reply(200, __doc__)
            return self.reply(404, "404\n")
        except ValueError as e:
            return self.reply(400, f"erro: {e}\n")

    def log_message(self, fmt, *args):  # só loga os /set, /del, /reset
        if not self.path.startswith(("/metrics", "/-/healthy")):
            print(f"[app] {self.command} {self.path}", flush=True)


if __name__ == "__main__":
    print("[app] ouvindo em :8000", flush=True)
    ThreadingHTTPServer(("0.0.0.0", 8000), H).serve_forever()
