"""
gameday-app: um "alvo de mentira" configurável, escrito UMA vez e reaproveitado
por todos os cenários de gameday.

- Lê /config/config.json:  {"ports": {"9182": {"profile": "basic", ...}, ...}}
  Cada porta vira um "pod" independente com o seu próprio /metrics.
- Na porta principal (a primeira da lista) também funciona como "pager" fake:
    POST /webhook    -> recebe notificações do Alertmanager
    GET  /received   -> lista o que chegou (JSON)
    POST /reset      -> esvazia a lista

Perfis (profile):
  basic        http_requests_total{code} ~10/s (error_ratio opcional), app_info,
               maintenance_mode opcional ("maintenance": true)
  flapping     como basic, mas 40% de erro em metade de cada ciclo de 30s
  cardinality  basic + app_requests_by_user_total{user_id} crescendo até 5000 séries
  resetting    http_requests_total 10/s que volta a zero a cada "reset_every" s
  latency      histograma http_request_duration_seconds (fast|slow via "speed")
  mismatch     http_requests_total{method} 10/s e http_errors_total{code} 2/s

Só stdlib: sem build, o container python:3.14-alpine roda o arquivo direto.
"""
import json
import os
import random
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

CONFIG = os.environ.get("APP_CONFIG", "/config/config.json")
BUCKETS = [0.05, 0.1, 0.25, 0.5, 1.0, 2.5]
received = []
received_lock = threading.Lock()


class Pod:
    def __init__(self, port, spec):
        self.port = port
        self.spec = spec
        self.profile = spec.get("profile", "basic")
        self.start = time.time()
        self.lock = threading.Lock()
        self.c = {}  # contadores: chave -> valor
        self.hist = {"count": 0.0, "sum": 0.0, "buckets": [0.0] * len(BUCKETS)}
        self.last_reset = self.start

    def inc(self, key, v):
        self.c[key] = self.c.get(key, 0.0) + v

    def tick(self, dt):
        now = time.time()
        el = now - self.start
        p, s = self.profile, self.spec
        with self.lock:
            if p in ("basic", "cardinality"):
                ratio = float(s.get("error_ratio", 0.0))
                self.inc('code="200"', 10 * dt * (1 - ratio))
                self.inc('code="500"', 10 * dt * ratio)
            elif p == "flapping":
                bad = (el % 30) < 15
                ratio = 0.4 if bad else 0.0
                self.inc('code="200"', 10 * dt * (1 - ratio))
                self.inc('code="500"', 10 * dt * ratio)
            elif p == "resetting":
                every = float(s.get("reset_every", 45))
                if now - self.last_reset >= every:
                    self.c = {}
                    self.last_reset = now
                self.inc('code="200"', 10 * dt)
            elif p == "mismatch":
                self.inc('method="GET"', 10 * dt)
                self.inc('code="500"', 2 * dt)
            elif p == "latency":
                slow = s.get("speed") == "slow"
                rps = float(s.get("rps", 10 if slow else 45))
                n = rps * dt
                # rápido: tudo em (0, 0.05]  |  lento: tudo em (0.5, 1]
                lo, hi = (0.5, 1.0) if slow else (0.0, 0.05)
                obs = (lo + hi) / 2
                self.hist["count"] += n
                self.hist["sum"] += n * obs
                for i, le in enumerate(BUCKETS):
                    if hi <= le:
                        self.hist["buckets"][i] += n

    def render(self):
        p, s = self.profile, self.spec
        out = []
        with self.lock:
            out.append("# HELP app_info Informação estática do app.")
            out.append("# TYPE app_info gauge")
            out.append(f'app_info{{version="1.4.2",profile="{p}"}} 1')
            if p in ("basic", "cardinality", "flapping", "resetting"):
                out.append("# HELP http_requests_total Requisições atendidas.")
                out.append("# TYPE http_requests_total counter")
                for k, v in sorted(self.c.items()):
                    out.append(f"http_requests_total{{{k}}} {int(v)}")
            if p == "mismatch":
                out.append("# HELP http_requests_total Requisições atendidas.")
                out.append("# TYPE http_requests_total counter")
                for k in ('method="GET"',):
                    out.append(f"http_requests_total{{{k}}} {int(self.c.get(k, 0))}")
                out.append("# HELP http_errors_total Requisições que falharam.")
                out.append("# TYPE http_errors_total counter")
                for k in ('code="500"',):
                    out.append(f"http_errors_total{{{k}}} {int(self.c.get(k, 0))}")
            if s.get("maintenance"):
                out.append("# HELP maintenance_mode 1 = sistema em janela de manutenção.")
                out.append("# TYPE maintenance_mode gauge")
                out.append('maintenance_mode{system="datalake"} 1')
            if p == "cardinality":
                el = time.time() - self.start
                n = int(min(5000, 500 + 60 * el))
                out.append("# HELP app_requests_by_user_total Requisições por usuário (debug!).")
                out.append("# TYPE app_requests_by_user_total counter")
                for u in range(n):
                    out.append(f'app_requests_by_user_total{{user_id="u{100000 + u}"}} {1 + (u % 7)}')
            if p == "latency":
                h = self.hist
                out.append("# HELP http_request_duration_seconds Latência das requisições.")
                out.append("# TYPE http_request_duration_seconds histogram")
                for le, v in zip(BUCKETS, h["buckets"]):
                    out.append(f'http_request_duration_seconds_bucket{{le="{le}"}} {v:.0f}')
                out.append(f'http_request_duration_seconds_bucket{{le="+Inf"}} {h["count"]:.0f}')
                out.append(f'http_request_duration_seconds_sum {h["sum"]:.3f}')
                out.append(f'http_request_duration_seconds_count {h["count"]:.0f}')
        return "\n".join(out) + "\n"


def make_handler(pod, is_main):
    class H(BaseHTTPRequestHandler):
        def log_message(self, *a):
            pass

        def send(self, code, body, ctype="text/plain; version=0.0.4; charset=utf-8"):
            b = body.encode()
            self.send_response(code)
            self.send_header("Content-Type", ctype)
            self.send_header("Content-Length", str(len(b)))
            self.end_headers()
            self.wfile.write(b)

        def do_GET(self):
            if self.path.startswith("/metrics"):
                return self.send(200, pod.render())
            if self.path.startswith("/received") and is_main:
                with received_lock:
                    return self.send(200, json.dumps(received, indent=1), "application/json")
            if self.path in ("/", "/healthz"):
                return self.send(200, "ok\n")
            self.send(404, "not found\n")

        def do_POST(self):
            n = int(self.headers.get("Content-Length", 0))
            body = self.rfile.read(n) if n else b""
            if self.path.startswith("/webhook") and is_main:
                try:
                    msg = json.loads(body)
                except ValueError:
                    return self.send(400, "bad json\n")
                with received_lock:
                    received.append({
                        "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                        "receiver": msg.get("receiver"),
                        "status": msg.get("status"),
                        "alerts": [{"status": a.get("status"), "labels": a.get("labels", {})}
                                   for a in msg.get("alerts", [])],
                    })
                    del received[:-200]
                return self.send(200, "ok\n")
            if self.path.startswith("/reset") and is_main:
                with received_lock:
                    received.clear()
                return self.send(200, "ok\n")
            self.send(404, "not found\n")

    return H


def main():
    with open(CONFIG) as f:
        cfg = json.load(f)
    ports = cfg.get("ports") or {"9182": {"profile": "basic"}}
    pods = []
    for i, (port, spec) in enumerate(ports.items()):
        pod = Pod(int(port), spec)
        pods.append(pod)
        srv = ThreadingHTTPServer(("0.0.0.0", int(port)), make_handler(pod, i == 0))
        threading.Thread(target=srv.serve_forever, daemon=True).start()
        print(f"pod :{port} profile={pod.profile} {spec}", flush=True)
    last = time.time()
    while True:
        time.sleep(0.25 + random.random() * 0.01)
        now = time.time()
        for pod in pods:
            pod.tick(now - last)
        last = now


if __name__ == "__main__":
    main()
