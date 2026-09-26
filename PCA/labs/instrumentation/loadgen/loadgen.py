"""Gerador de carga: bate nos apps o tempo todo para as métricas terem dados."""
import os
import random
import threading
import time
import urllib.error
import urllib.request

TARGETS = os.environ.get("TARGETS", "http://app-go:8080,http://app-go-b:8080,http://app-python:8080").split(",")
THREADS = int(os.environ.get("THREADS_PER_TARGET", "4"))


def worker(base):
    while True:
        route = "/api/checkout" if random.random() < 0.5 else "/api/items"
        # user_id aleatório: alimenta a bomba de cardinalidade http_requests_by_user_total
        req = urllib.request.Request(base + route, headers={"X-User-Id": str(random.randint(1, 5000))})
        try:
            urllib.request.urlopen(req, timeout=5).read()
        except urllib.error.HTTPError:
            pass  # 402/500 fazem parte do cenário
        except Exception:
            time.sleep(1)  # app ainda subindo
        time.sleep(random.uniform(0.05, 0.25))


for t in TARGETS:
    for _ in range(THREADS):
        threading.Thread(target=worker, args=(t,), daemon=True).start()
print("loadgen:", TARGETS, "x", THREADS, "threads")
while True:
    time.sleep(3600)
