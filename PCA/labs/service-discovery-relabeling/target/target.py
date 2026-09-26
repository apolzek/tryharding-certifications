"""Alvo fake para o lab de service discovery e relabeling (só stdlib).

Uma única imagem (python:3.14-alpine) roda N vezes; o comportamento vem de env vars:

  SERVICE        nome do serviço (vira label service= em app_info)
  TEAM           time dono (vira label team= em app_info)
  VERSION        versão (default 1.0.0)
  METRICS_PATH   caminho das métricas (default /metrics)
  HIGH_CARD      N séries de app_requests_by_user_total{user_id=...} (default 0)
  JOB_LABEL      se definido, expõe métricas de batch COM label job="<valor>"
                 (para estudar honor_labels / exported_job)
  SD_FILE        se definido, serve esse arquivo em /targets.json (http_sd)

Endpoints extras:
  /probe?target=host:porta   estilo blackbox: tenta GET http://<target>/metrics
  /targets.json              conteúdo de SD_FILE (http_sd_configs)
"""
import os
import threading
import time
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

START = time.time()
SERVICE = os.environ.get("SERVICE", "app")
TEAM = os.environ.get("TEAM", "none")
VERSION = os.environ.get("VERSION", "1.0.0")
METRICS_PATH = os.environ.get("METRICS_PATH", "/metrics")
HIGH_CARD = int(os.environ.get("HIGH_CARD", "0"))
JOB_LABEL = os.environ.get("JOB_LABEL", "")
SD_FILE = os.environ.get("SD_FILE", "")
ROUTES = {"/": 5.0, "/api/pay": 2.0, "/health": 1.0}


def metrics(params):
    up = time.time() - START
    out = [
        "# HELP app_info Informações estáticas do serviço.",
        "# TYPE app_info gauge",
        f'app_info{{service="{SERVICE}",team="{TEAM}",version="{VERSION}"}} 1',
        "# HELP app_requests_total Requisições atendidas.",
        "# TYPE app_requests_total counter",
    ]
    for route, rps in ROUTES.items():
        out.append(f'app_requests_total{{route="{route}"}} {int(up * rps)}')
    out += [
        "# HELP app_cache_entries Itens no cache (label pod_uid é ruído).",
        "# TYPE app_cache_entries gauge",
        f'app_cache_entries{{cache="sessions",pod_uid="{SERVICE}-7f9c-{int(START) % 10000}"}} 42',
    ]
    if HIGH_CARD:
        out += [
            "# HELP app_requests_by_user_total Requisições por usuário (ALTA CARDINALIDADE!).",
            "# TYPE app_requests_by_user_total counter",
        ]
        out += [f'app_requests_by_user_total{{user_id="u{i:05d}"}} {i}' for i in range(HIGH_CARD)]
    if JOB_LABEL:
        ts_ms = int((time.time() - 30) * 1000)
        out += [
            "# HELP batch_last_success_timestamp_seconds Último sucesso do job batch.",
            "# TYPE batch_last_success_timestamp_seconds gauge",
            f'batch_last_success_timestamp_seconds{{job="{JOB_LABEL}"}} {int(START)}',
            "# HELP batch_records_processed_total Registros processados pelo batch.",
            "# TYPE batch_records_processed_total counter",
            f'batch_records_processed_total{{job="{JOB_LABEL}"}} {int(up * 3)}',
            "# HELP batch_heartbeat Amostra com timestamp explícito (30s atrás).",
            "# TYPE batch_heartbeat gauge",
            f"batch_heartbeat 1 {ts_ms}",
        ]
    if params:
        out += ["# HELP app_scrape_param Parâmetros de URL recebidos no scrape.",
                "# TYPE app_scrape_param gauge"]
        for k, vs in sorted(params.items()):
            for v in vs:
                out.append(f'app_scrape_param{{name="{k}",value="{v}"}} 1')
    return "\n".join(out) + "\n"


def probe(target):
    t0 = time.time()
    res = {"ok": 0}

    def check():
        try:
            with urllib.request.urlopen(f"http://{target}/metrics", timeout=1) as r:
                res["ok"] = 1 if r.status == 200 else 0
        except Exception:
            pass

    # DNS inexistente pode demorar ~5s; desiste em 1.5s (como o timeout do blackbox)
    th = threading.Thread(target=check, daemon=True)
    th.start()
    th.join(1.5)
    ok = res["ok"]
    dur = time.time() - t0
    return (
        "# HELP probe_success 1 se o alvo respondeu.\n# TYPE probe_success gauge\n"
        f"probe_success {ok}\n"
        "# HELP probe_duration_seconds Duração do probe.\n# TYPE probe_duration_seconds gauge\n"
        f"probe_duration_seconds {dur:.4f}\n"
    )


class H(BaseHTTPRequestHandler):
    def _send(self, code, body, ctype="text/plain; version=0.0.4; charset=utf-8"):
        b = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)

    def do_GET(self):
        u = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qs(u.query)
        if u.path == METRICS_PATH:
            self._send(200, metrics(params))
        elif u.path == "/probe":
            target = params.get("target", [""])[0]
            if not target:
                self._send(400, "parâmetro target ausente\n")
            else:
                self._send(200, probe(target))
        elif u.path == "/targets.json" and SD_FILE:
            try:
                with open(SD_FILE) as f:
                    self._send(200, f.read(), "application/json")
            except OSError as e:
                self._send(500, f"{e}\n")
        elif u.path == "/healthz":
            self._send(200, "ok\n")
        else:
            self._send(404, f"404: tente {METRICS_PATH}\n")

    def log_message(self, *a):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8000), H).serve_forever()
