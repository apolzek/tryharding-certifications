"""GABARITO do app-python: exercícios 01, 02, 04, 05 e 06 aplicados.
Procure por "EX0x" para ver cada mudança.
(O 03 é só no Go: o Summary do client Python nem calcula quantis.)
"""
import os
import random
import secrets
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from prometheus_client import REGISTRY, CollectorRegistry, Counter, Gauge, Histogram, Info, Summary
from prometheus_client.exposition import choose_encoder

BAD = CollectorRegistry()  # agora só tem métricas corrigidas

HTTP_REQUESTS = Counter("http_requests_total", "Total de requisições HTTP atendidas.",
                        ["method", "route", "code"])
# EX02: bucket exatamente no alvo do SLO (0.3s) e resolução em volta dele.
HTTP_DURATION = Histogram("http_request_duration_seconds", "Latência das requisições HTTP.",
                          ["method", "route"],
                          buckets=[0.025, 0.05, 0.1, 0.2, 0.3, 0.45, 0.6, 1, 2.5])
IN_FLIGHT = Gauge("http_requests_in_flight", "Requisições sendo atendidas agora.")
BACKEND_DURATION = Summary("backend_call_duration_seconds",
                           "Latência das chamadas ao backend de estoque.")

# EX05: Info -> app_info{version=...,commit=...,language=...} 1
APP_INFO = Info("app", "Metadados da aplicação.")
APP_INFO.info({"version": "1.4.2", "commit": "9f3c2ab", "language": "python"})

# EX06: counter + histogram para a função de pagamento
PAYMENTS = Counter("payments_total", "Pagamentos processados, por resultado.", ["result"])
PAYMENT_DURATION = Histogram("payment_duration_seconds", "Duração do processamento de pagamento.",
                             buckets=[0.005, 0.01, 0.02, 0.03, 0.04, 0.05, 0.1])

# EX01: nomes corrigidos
CHECKOUTS = Counter("checkouts_total", "Checkouts realizados.", registry=BAD)
CHECKOUT_DURATION = Histogram("checkout_duration_seconds", "Latência do checkout.",
                              buckets=[0.05, 0.1, 0.25, 0.5, 1], registry=BAD)
ORDERS_PROCESSED = Counter("orders_processed_total", "Pedidos processados.", registry=BAD)
CACHE_SIZE = Gauge("cache_size_bytes", "Tamanho do cache.", registry=BAD)
CART_ITEMS = Counter("cart_items_total", "Itens adicionados ao carrinho.", registry=BAD)
# EX04: user_id -> plan (cardinalidade 3)
CHECKOUTS_BY_PLAN = Counter("checkouts_by_plan_total", "Checkouts por plano do cliente.",
                            ["plan"], registry=BAD)

LATENCY_FACTOR = float(os.environ.get("BACKEND_LATENCY_FACTOR", "1"))


class Multi:
    def __init__(self, *regs):
        self.regs = regs

    def collect(self):
        for r in self.regs:
            yield from r.collect()


REGISTRIES = {"/metrics": Multi(REGISTRY, BAD), "/metrics/good": REGISTRY, "/metrics/bad": BAD}


def sleep_ms(lo, hi):
    time.sleep(random.uniform(lo, hi) / 1000)


def plan_of(user_id):
    try:
        n = int(user_id)
    except ValueError:
        n = 0
    return ("free", "pro", "enterprise")[n % 3]


class Declined(Exception):
    pass


@PAYMENT_DURATION.time()  # EX06: o decorator mede a duração
def process_payment():
    sleep_ms(5, 40)
    if random.random() < 0.10:
        PAYMENTS.labels("declined").inc()
        raise Declined("card declined")
    PAYMENTS.labels("success").inc()


def call_backend():
    with BACKEND_DURATION.time():
        sleep_ms(10 * LATENCY_FACTOR, 50 * LATENCY_FACTOR)


def sync_inventory():
    while True:
        call_backend()
        time.sleep(0.02)


def items(h):
    sleep_ms(5, 50)
    CACHE_SIZE.set((1024 + random.randint(0, 511)) * 1024)  # EX01: bytes
    return 200, "items ok\n"


def checkout(h):
    start = time.perf_counter()
    CHECKOUTS_BY_PLAN.labels(plan_of(h.headers.get("X-User-Id", ""))).inc()
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
    CHECKOUTS.inc()
    ORDERS_PROCESSED.inc()
    CHECKOUT_DURATION.observe(time.perf_counter() - start)  # EX01: segundos
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
            encoder, ctype = choose_encoder(self.headers.get("Accept"))
            return self.reply(200, encoder(REGISTRIES[path]), ctype)
        if path == "/healthz":
            return self.reply(200, "ok\n")
        fn = ROUTES.get(path)
        if fn is None:
            return self.reply(404, "not found\n")
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
    print("app-python (gabarito) ouvindo em :8080")
    ThreadingHTTPServer(("", 8080), Handler).serve_forever()
