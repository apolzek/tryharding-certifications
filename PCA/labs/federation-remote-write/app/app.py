"""App fake (só stdlib) para o lab de federation/remote write.

Env vars:
  APP   nome (vira label app= em app_build_info)
  RPS   requisições/s na rota "/" (as outras rotas são frações disso)
"""
import os
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

START = time.time()
APP = os.environ.get("APP", "app")
RPS = float(os.environ.get("RPS", "10"))
# (route, code, fração do RPS)
SERIES = [("/", "200", 1.0), ("/", "500", 0.05), ("/checkout", "200", 0.3),
          ("/checkout", "500", 0.03), ("/search", "200", 0.5), ("/search", "500", 0.01)]


def metrics():
    up = time.time() - START
    out = [
        "# HELP app_build_info Versão do app.",
        "# TYPE app_build_info gauge",
        f'app_build_info{{app="{APP}",version="1.4.2"}} 1',
        "# HELP http_requests_total Requisições HTTP atendidas.",
        "# TYPE http_requests_total counter",
    ]
    for route, code, frac in SERIES:
        out.append(f'http_requests_total{{route="{route}",code="{code}"}} {int(up * RPS * frac)}')
    out += [
        "# HELP app_inflight_requests Requisições em andamento.",
        "# TYPE app_inflight_requests gauge",
        f"app_inflight_requests {int(RPS) % 7 + 1}",
    ]
    return "\n".join(out) + "\n"


class H(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.startswith("/metrics"):
            body, code = metrics().encode(), 200
        else:
            body, code = b"try /metrics\n", 404
        self.send_response(code)
        self.send_header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *a):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8000), H).serve_forever()
