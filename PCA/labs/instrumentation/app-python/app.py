"""app-python: a mesma loja fake, instrumentada com prometheus_client.

Endpoints de métricas:
  /metrics       -> tudo (BOAS + RUINS), é o que o Prometheus raspa
  /metrics/good  -> só as boas (promtool check metrics passa)
  /metrics/bad   -> só as ruins (promtool check metrics reclama)
"""
import os
import random
import secrets
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from prometheus_client import REGISTRY, CollectorRegistry, Counter, Gauge, Histogram, Summary
from prometheus_client.exposition import choose_encoder

BAD = CollectorRegistry()

# ─────────────── BOAS (no REGISTRY padrão, que já traz process_*, python_gc_*, python_info) ───────────────

# COUNTER: o client Python SEMPRE expõe com _total (e também _created).
HTTP_REQUESTS = Counter("http_requests_total", "Total de requisições HTTP atendidas.",
                        ["method", "route", "code"])
# HISTOGRAM em SEGUNDOS. O client Python (0.26) só expõe buckets clássicos.
HTTP_DURATION = Histogram("http_request_duration_seconds", "Latência das requisições HTTP.",
                          ["method", "route"])  # buckets padrão = .005 .. 10
IN_FLIGHT = Gauge("http_requests_in_flight", "Requisições sendo atendidas agora.")
# SUMMARY no Python: só _count e _sum, SEM quantis!
BACKEND_DURATION = Summary("backend_call_duration_seconds",
                           "Latência das chamadas ao backend de estoque.")

# ─────────────── RUINS (exercícios 01 e 04) ───────────────

# RUIM: camelCase (o client Python força o sufixo _total -> requestsCount_total).
REQUESTS_COUNT = Counter("requestsCount", "Checkouts realizados.", registry=BAD)
# RUIM: milissegundos (e abreviado).
CHECKOUT_LATENCY_MS = Histogram("checkout_latency_ms", "Latência do checkout em milissegundos.",
                                buckets=[50, 100, 250, 500, 1000], registry=BAD)
# RUIM: gauge usado como counter, com _total.
ORDERS_PROCESSED = Gauge("orders_processed_total", "Pedidos processados.", registry=BAD)
# RUIM: unidade abreviada e não-base.
CACHE_SIZE_KB = Gauge("cache_size_kb", "Tamanho do cache em kilobytes.", registry=BAD)
# RUIM: sem HELP.
CART_ITEMS = Counter("cart_items", "", registry=BAD)
# RUIM (o promtool NÃO pega!): bomba de cardinalidade.
REQUESTS_BY_USER = Counter("http_requests_by_user_total", "Requisições por usuário.",
                           ["user_id"], registry=BAD)

LATENCY_FACTOR = float(os.environ.get("BACKEND_LATENCY_FACTOR", "1"))


class Multi:
    """Junta vários registries num só (é o que o /metrics serve)."""

    def __init__(self, *regs):
        self.regs = regs

    def collect(self):
        for r in self.regs:
            yield from r.collect()


REGISTRIES = {"/metrics": Multi(REGISTRY, BAD), "/metrics/good": REGISTRY, "/metrics/bad": BAD}


def sleep_ms(lo, hi):
    time.sleep(random.uniform(lo, hi) / 1000)


class Declined(Exception):
    pass


def process_payment():
    """Função de negócio SEM instrumentação (exercício 06)."""
    sleep_ms(5, 40)
    if random.random() < 0.10:
        raise Declined("card declined")


def call_backend():
    with BACKEND_DURATION.time():
        sleep_ms(10 * LATENCY_FACTOR, 50 * LATENCY_FACTOR)


def sync_inventory():
    """Worker em background que chama o backend de estoque o tempo todo."""
    while True:
        call_backend()
        time.sleep(0.02)


def items(h):
    sleep_ms(5, 50)
    CACHE_SIZE_KB.set(1024 + random.randint(0, 511))
    return 200, "items ok\n"


def checkout(h):
    start = time.perf_counter()
    REQUESTS_BY_USER.labels(h.headers.get("X-User-Id", "")).inc()
    CART_ITEMS.inc(random.randint(1, 4))
    if random.random() < 0.07:
        sleep_ms(350, 600)
    else:
        sleep_ms(20, 200)
    if random.random() < 0.05:
        return 500, "boom\n"
    try:
        process_payment()
    except Declined as e:
        return 402, f"{e}\n"
    REQUESTS_COUNT.inc()
    ORDERS_PROCESSED.inc()
    CHECKOUT_LATENCY_MS.observe((time.perf_counter() - start) * 1000)
    return 200, "checkout ok\n"


ROUTES = {"/api/items": items, "/api/checkout": checkout}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, code, body, ctype="text/plain"):
        data = body if isinstance(body, bytes) else body.encode()
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        path = self.path.split("?")[0]
        if path in REGISTRIES:
            # negociação de formato: text 0.0.4 ou OpenMetrics, conforme o Accept
            encoder, ctype = choose_encoder(self.headers.get("Accept"))
            return self.reply(200, encoder(REGISTRIES[path]), ctype)
        if path == "/healthz":
            return self.reply(200, "ok\n")
        fn = ROUTES.get(path)
        if fn is None:
            return self.reply(404, "not found\n")
        # middleware RED + exemplar
        IN_FLIGHT.inc()
        start = time.perf_counter()
        try:
            code, body = fn(self)
        finally:
            IN_FLIGHT.dec()
        HTTP_REQUESTS.labels("GET", path, str(code)).inc()
        HTTP_DURATION.labels("GET", path).observe(time.perf_counter() - start,
                                                  exemplar={"trace_id": secrets.token_hex(16)})
        self.reply(code, body)


if __name__ == "__main__":
    threading.Thread(target=sync_inventory, daemon=True).start()
    print("app-python ouvindo em :8080")
    ThreadingHTTPServer(("", 8080), Handler).serve_forever()
