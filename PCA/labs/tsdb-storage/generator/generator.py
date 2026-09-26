"""Gerador de cardinalidade controlável para o lab tsdb-storage (só stdlib).

Endpoints:
  GET /metrics                      exposição Prometheus (text format 0.0.4)
  GET /control                      mostra o estado atual (JSON)
  GET /control?users=500            muda o nº de séries de tsdb_demo_requests_total
  GET /control?ephemeral=0|1        liga/desliga a série tsdb_demo_ephemeral (alvo continua vivo)
  GET /control?timestamped=0|1      liga/desliga tsdb_demo_timestamped (exposta COM timestamp explícito)
  GET /control?down=0|1             1 = /metrics responde 503 (simula alvo fora do ar)
  GET /control?ts_offset=600        tsdb_demo_timestamped passa a ser exposta 600s NO PASSADO (out-of-order)
  GET /backfill.om                  1 dia de dados HISTÓRICOS em OpenMetrics (para promtool tsdb create-blocks-from)
                                    ?end=<unix>  fim dos dados (padrão: hora cheia de 2h atrás)
                                    ?hours=24    duração   ?step=60  intervalo entre amostras (s)
"""
import json
import random
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

STATE = {"users": 10, "ephemeral": 1, "timestamped": 1, "down": 0, "ts_offset": 0}
PATHS = ["/home", "/cart", "/checkout", "/search"]
ROOMS = ["kitchen", "living", "office"]
START = time.time()


def render() -> str:
    now = time.time()
    up_s = now - START
    out = []
    out.append("# HELP tsdb_demo_requests_total Requisições por usuário (cardinalidade controlada por ?users=N).")
    out.append("# TYPE tsdb_demo_requests_total counter")
    for u in range(STATE["users"]):
        path = PATHS[u % len(PATHS)]
        # cada série cresce num ritmo diferente, mas determinístico
        out.append(f'tsdb_demo_requests_total{{path="{path}",user_id="u{u:05d}"}} {int(up_s * (1 + u % 5))}')

    out.append("# HELP tsdb_demo_temperature_celsius Temperatura por cômodo (cardinalidade baixa e fixa).")
    out.append("# TYPE tsdb_demo_temperature_celsius gauge")
    for i, r in enumerate(ROOMS):
        out.append(f'tsdb_demo_temperature_celsius{{room="{r}"}} {20 + i + random.random():.2f}')

    if STATE["ephemeral"]:
        out.append("# HELP tsdb_demo_ephemeral Série que some do /metrics quando ephemeral=0 (o alvo continua UP).")
        out.append("# TYPE tsdb_demo_ephemeral gauge")
        out.append('tsdb_demo_ephemeral{kind="no_timestamp"} 1')

    if STATE["timestamped"]:
        # Timestamp explícito (ms): o Prometheus NÃO grava stale marker quando ela some.
        out.append("# HELP tsdb_demo_timestamped Série exposta com timestamp explícito (estilo federação/pushgateway antigo).")
        out.append("# TYPE tsdb_demo_timestamped gauge")
        out.append(f'tsdb_demo_timestamped{{kind="with_timestamp"}} 1 {int((now - STATE["ts_offset"]) * 1000)}')

    out.append("# HELP tsdb_demo_config_users Valor atual de ?users.")
    out.append("# TYPE tsdb_demo_config_users gauge")
    out.append(f"tsdb_demo_config_users {STATE['users']}")
    return "\n".join(out) + "\n"


def backfill_end(now=None) -> int:
    """Fim padrão do backfill: hora cheia, 2h atrás (longe da head, que cobre ~as últimas 2-3h)."""
    now = int(now or time.time())
    return (now // 3600) * 3600 - 2 * 3600


def render_backfill(end: int, hours: int, step: int) -> str:
    """OpenMetrics: famílias contíguas, amostras em ordem crescente de tempo, timestamps em SEGUNDOS, termina com # EOF."""
    start = end - hours * 3600
    ts = list(range(start, end, step))
    out = []
    # counter: pedidos por loja, com "horário comercial" (mais pedidos entre 9h e 18h UTC)
    out.append("# HELP tsdb_backfill_orders Pedidos por loja (dados históricos importados).")
    out.append("# TYPE tsdb_backfill_orders counter")
    for store, base in (("sp", 3), ("rj", 1)):
        total = 0
        for t in ts:
            hour = (t // 3600) % 24
            total += base * step * (3 if 9 <= hour < 18 else 1) // 60
            out.append(f'tsdb_backfill_orders_total{{store="{store}"}} {total} {t}')
    # gauge "denso": 1 amostra por step
    out.append("# HELP tsdb_backfill_temperature_celsius Temperatura por cidade (dados históricos).")
    out.append("# TYPE tsdb_backfill_temperature_celsius gauge")
    for city, base in (("sao_paulo", 22), ("curitiba", 15)):
        for t in ts:
            hour = (t // 3600) % 24
            out.append(f'tsdb_backfill_temperature_celsius{{city="{city}"}} {base + (hour - 12) / 2:.1f} {t}')
    # gauge ESPARSO: 1 amostra a cada 30 min, valor = hora do dia (UTC). Bom para ver o lookback delta.
    out.append("# HELP tsdb_backfill_sparse_gauge Uma amostra a cada 30 min; valor = hora UTC da amostra.")
    out.append("# TYPE tsdb_backfill_sparse_gauge gauge")
    for t in range(start, end, 1800):
        out.append(f"tsdb_backfill_sparse_gauge {(t // 3600) % 24} {t}")
    out.append("# EOF")
    return "\n".join(out) + "\n"


class Handler(BaseHTTPRequestHandler):
    def _send(self, code, body, ctype="text/plain; version=0.0.4; charset=utf-8"):
        data = body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        u = urlparse(self.path)
        if u.path == "/metrics":
            if STATE["down"]:
                return self._send(503, "down (simulado)\n")
            return self._send(200, render())
        if u.path == "/control":
            q = parse_qs(u.query)
            for k in STATE:
                if k in q:
                    v = int(q[k][0])
                    if k == "users":
                        STATE[k] = max(0, min(v, 200000))
                    elif k == "ts_offset":
                        STATE[k] = max(0, min(v, 86400))
                    else:
                        STATE[k] = 1 if v else 0
            return self._send(200, json.dumps(STATE) + "\n", "application/json")
        if u.path == "/backfill.om":
            q = parse_qs(u.query)
            end = int(q.get("end", [backfill_end()])[0])
            hours = int(q.get("hours", [24])[0])
            step = int(q.get("step", [60])[0])
            return self._send(200, render_backfill(end, hours, step),
                              "application/openmetrics-text; version=1.0.0; charset=utf-8")
        if u.path in ("/", "/healthz"):
            return self._send(200, "ok\n")
        return self._send(404, "not found\n")

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8000), Handler).serve_forever()
