#!/usr/bin/env python3
"""Corretor automático dos desafios PromQL.

Uso:
  ./check.py list [--topic operators] [--level basic]   # lista desafios (✅ = resolvido)
  ./check.py show 001                                   # mostra o enunciado
  ./check.py 001 'sum(rate(x[1m]))'                     # confere sua resposta
  ./check.py 001 -f resposta.promql                     # resposta vinda de arquivo
  ./check.py hint 001                                   # próxima dica
  ./check.py solution 001                               # mostra gabarito + explicação
  ./check.py progress                                   # resumo por tópico
  ./check.py exporter                                   # expõe seu progresso em :9199/metrics
  ./check.py exporter --docker                          # idem, num container (se o firewall bloquear o host)
  ./check.py selftest [ids|tópicos]                     # (CI) todo gabarito roda e bate consigo mesmo

A correção compara o RESULTADO (séries + valores) da sua query com o do gabarito,
avaliados no MESMO instante — então soluções diferentes mas equivalentes passam.

Formato de um desafio (YAML em challenges/<tópico>/<id>-<slug>.yaml):
  id: "001"                 # único, 3 dígitos
  title: "..."
  topic: operators           # pasta
  level: basic|intermediate|advanced
  domain: "PromQL"           # domínio da PCA
  lesson: "../promql-functions-lab/functions/rate/"   # opcional, link de estudo
  prompt: |                  # enunciado (pt-BR), cite as métricas a usar
  hints: ["...", "..."]
  solution: 'promql'
  explanation: |             # por que o gabarito funciona / pegadinhas
  compare: values            # values (default) | labels | count | scalar
  tolerance: 0.02            # tolerância relativa p/ values (default 0.02)
  ignore_labels: []          # labels ignorados na comparação (__name__ é sempre ignorado)
  ordered: false             # true => a ORDEM das séries importa (sort/topk)
  expect_empty: false        # true => gabarito legitimamente vazio (ex.: absent)
"""
import argparse, json, math, os, pathlib, re, subprocess, sys, threading, time, http.server
try:  # o exporter roda só com a stdlib (ex.: python:3.14-alpine puro); o resto precisa dos dois
    import requests
except ImportError:
    requests = None
try:
    import yaml
except ImportError:
    yaml = None

ROOT = pathlib.Path(__file__).resolve().parent
PROM = os.environ.get("PROM_URL", "http://localhost:9095")
PROGRESS = pathlib.Path(os.environ.get("PCA_PROGRESS", ROOT / ".progress.json"))
# Avalia alguns segundos no passado: evita que um scrape "em voo" (amostra com timestamp
# anterior ao instante, mas gravada entre a query do gabarito e a sua) mude o resultado.
EVAL_DELAY = float(os.environ.get("PCA_EVAL_DELAY", 10))
C = {"g": "\033[32m", "r": "\033[31m", "y": "\033[33m", "b": "\033[1m", "d": "\033[2m", "x": "\033[0m"}
if not sys.stdout.isatty():
    C = {k: "" for k in C}


def challenge_files():
    # pastas que começam com "_" ou "." (ex.: _example) não contam
    return [f for f in sorted(ROOT.glob("*/*.yaml")) if not f.parent.name.startswith(("_", "."))]


def load_meta():
    """Só id/topic/level, sem PyYAML (usado pelo exporter)."""
    out = {}
    for f in challenge_files():
        m = dict(re.findall(r"^(id|topic|level):\s*['\"]?([^'\"\n]+?)['\"]?\s*$", f.read_text(), re.M))
        if "id" in m:
            out[m["id"]] = {"id": m["id"], "topic": m.get("topic", f.parent.name), "level": m.get("level", "?")}
    return dict(sorted(out.items()))


def load_all():
    if yaml is None or requests is None:
        sys.exit("faltam dependências: pip install pyyaml requests")
    out = {}
    for f in challenge_files():
        c = yaml.safe_load(f.read_text())
        c["_file"] = f
        if c["id"] in out:
            sys.exit(f"id duplicado {c['id']}: {f} e {out[c['id']]['_file']}")
        out[c["id"]] = c
    return dict(sorted(out.items()))  # ordem de estudo = ordem dos ids


def get(ch, cid):
    cid = cid.zfill(3)
    if cid not in ch:
        sys.exit(f"desafio {cid} não existe (veja ./check.py list)")
    return ch[cid]


def progress():
    try:
        return json.loads(PROGRESS.read_text())
    except Exception:
        return {"solved": {}, "attempts": {}, "hints": {}}


def save(p):
    PROGRESS.write_text(json.dumps(p, indent=1))


_tls = threading.local()


def _session():
    # conexão keep-alive (por thread): evita abrir milhares de sockets no selftest
    if not hasattr(_tls, "s"):
        _tls.s = requests.Session()
    return _tls.s


def query(expr, ts):
    for attempt in range(5):  # tolera o Prometheus/porta piscando (ex.: outro compose subindo)
        try:
            r = _session().post(f"{PROM}/api/v1/query", data={"query": expr, "time": ts}, timeout=30)
            break
        except requests.ConnectionError:
            if attempt == 4:
                raise SystemExit(f"não consegui falar com o Prometheus em {PROM}. "
                                 "O promql-functions-lab está no ar? (cd ../promql-functions-lab && docker compose up -d)")
            time.sleep(2)
    j = r.json()
    if j.get("status") != "success":
        raise ValueError(f"{j.get('errorType')}: {j.get('error')}")
    return j["data"]


def normalize(data, ignore):
    rt = data["resultType"]
    if rt == "scalar":
        return "scalar", [((), float(data["result"][1]))]
    if rt == "string":
        return "string", [((), data["result"][1])]
    rows = []
    for s in data["result"]:
        lbl = tuple(sorted((k, v) for k, v in s["metric"].items() if k != "__name__" and k not in ignore))
        if rt == "vector" and "value" in s:
            v = float(s["value"][1])
        elif rt == "vector":  # native histogram: compara contagem e soma
            h = s["histogram"][1]
            v = ("histogram", float(h.get("count", 0)), float(h.get("sum", 0)))
        else:  # range vector: compara nº de amostras e o último valor
            vals = s.get("values") or s.get("histograms") or []
            last = vals[-1][1] if vals else "nan"
            v = ("range-vector", float(len(vals)), float(last) if not isinstance(last, dict) else float(last.get("count", 0)))
        rows.append((lbl, v))
    return rt, rows


def fmt(v):
    if isinstance(v, tuple):
        return f"<{v[0]}>"
    return f"{v:.4g}" if isinstance(v, float) else str(v)


def close(a, b, tol):
    if isinstance(a, tuple) or isinstance(b, tuple):
        return (isinstance(a, tuple) and isinstance(b, tuple) and len(a) == len(b) and a[0] == b[0]
                and all(close(x, y, tol) for x, y in zip(a[1:], b[1:])))
    if isinstance(a, str) or isinstance(b, str):
        return a == b
    if math.isnan(a) and math.isnan(b):
        return True
    if math.isinf(a) or math.isinf(b):
        return a == b
    return abs(a - b) <= tol * max(abs(a), abs(b), 1e-9) or abs(a - b) < 1e-9


def compare(c, got, want):
    mode, tol = c.get("compare", "values"), c.get("tolerance", 0.02)
    ign = set(c.get("ignore_labels", []))
    gt, g = normalize(got, ign)
    wt, w = normalize(want, ign)
    if c.get("expect_empty") and not w and not g:
        return True, "resultado vazio, como esperado"
    if not w:
        return False, "o gabarito voltou vazio agora (dados ainda chegando?). Tente de novo em 1 min."
    if gt != wt and "scalar" not in (gt, wt):
        return False, f"tipo de resultado errado: veio {gt}, esperado {wt}"
    if mode == "scalar" or wt == "scalar":
        if gt != wt:
            return False, f"esperado tipo {wt}, veio {gt}"
        return (close(g[0][1], w[0][1], tol), f"valor {fmt(g[0][1])} vs esperado {fmt(w[0][1])}")
    if mode == "count":
        return len(g) == len(w), f"{len(g)} séries (esperado {len(w)})"
    gk, wk = [x[0] for x in g], [x[0] for x in w]
    if set(gk) != set(wk):
        extra, miss = set(gk) - set(wk), set(wk) - set(gk)
        msg = []
        if miss:
            msg.append(f"faltam {len(miss)} série(s), ex.: {{{', '.join(f'{k}={v!r}' for k, v in sorted(miss)[0])}}}")
        if extra:
            msg.append(f"sobram {len(extra)} série(s), ex.: {{{', '.join(f'{k}={v!r}' for k, v in sorted(extra)[0])}}}")
        return False, "labels diferentes: " + "; ".join(msg)
    if c.get("ordered") and gk != wk:
        return False, "séries certas, mas na ORDEM errada"
    if mode == "labels":
        return True, "labels corretos"
    wd = dict(w)
    for k, v in g:
        if not close(v, wd[k], tol):
            if isinstance(v, tuple) or isinstance(wd[k], tuple):
                if not (isinstance(v, tuple) and isinstance(wd[k], tuple) and v[0] == wd[k][0]):
                    return False, f"tipo errado em {dict(k) or '{}'}: veio {fmt(v)}, esperado {fmt(wd[k])}"
            return False, f"valor errado em {dict(k) or '{}'}: {fmt(v)} (esperado ≈ {fmt(wd[k])})"
    return True, f"{len(g)} série(s) batendo"


def cmd_list(ch, a):
    p = progress()
    cur = None
    for cid, c in ch.items():
        if a.topic and c["topic"] != a.topic or a.level and c["level"] != a.level:
            continue
        if c["topic"] != cur:
            cur = c["topic"]
            tot = [k for k, v in ch.items() if v["topic"] == cur]
            print(f"\n{C['b']}{cur}{C['x']} {C['d']}({sum(k in p['solved'] for k in tot)}/{len(tot)}){C['x']}")
        mark = f"{C['g']}✅{C['x']}" if cid in p["solved"] else "⬜"
        lv = {"basic": "🟢", "intermediate": "🟡", "advanced": "🔴"}.get(c["level"], "")
        print(f"  {mark} {cid} {lv} {c['title']}")


def cmd_show(c):
    lv = {"basic": "🟢 básico", "intermediate": "🟡 intermediário", "advanced": "🔴 avançado"}[c["level"]]
    print(f"{C['b']}[{c['id']}] {c['title']}{C['x']}  ({lv} · {c['topic']} · {c.get('domain', 'PromQL')})\n")
    print(c["prompt"].rstrip())
    if c.get("lesson"):
        print(f"\n{C['d']}📘 estude: {c['lesson']}{C['x']}")
    print(f"\n{C['d']}responda: ./check.py {c['id']} '<sua query>'{C['x']}")


def cmd_check(c, answer):
    p = progress()
    p["attempts"][c["id"]] = p["attempts"].get(c["id"], 0) + 1
    ts = time.time() - EVAL_DELAY
    try:
        want = query(c["solution"], ts)
    except Exception as e:
        sys.exit(f"erro no gabarito (avise o mantenedor): {e}")
    try:
        got = query(answer, ts)
    except Exception as e:
        save(p)
        print(f"{C['r']}✘ sua query não roda:{C['x']} {e}")
        return 1
    ok, why = compare(c, got, want)
    if ok:
        first = c["id"] not in p["solved"]
        p["solved"][c["id"]] = {"ts": ts, "attempts": p["attempts"][c["id"]]}
        save(p)
        print(f"{C['g']}✔ CORRETO!{C['x']} {why}")
        if first:
            print(f"\n{C['b']}Por que funciona:{C['x']}\n{c.get('explanation', '').rstrip()}")
            print(f"\n{C['d']}gabarito de referência: {c['solution']}{C['x']}")
        return 0
    save(p)
    print(f"{C['r']}✘ ainda não:{C['x']} {why}")
    print(f"{C['d']}dica: ./check.py hint {c['id']}{C['x']}")
    return 1


def cmd_hint(c):
    p = progress()
    n = p["hints"].get(c["id"], 0)
    hints = c.get("hints", [])
    if n >= len(hints):
        print("sem mais dicas. Última opção: ./check.py solution", c["id"])
        return
    print(f"💡 dica {n + 1}/{len(hints)}: {hints[n]}")
    p["hints"][c["id"]] = n + 1
    save(p)


def cmd_progress(ch):
    p = progress()
    by = {}
    for cid, c in ch.items():
        t = by.setdefault(c["topic"], [0, 0])
        t[1] += 1
        t[0] += cid in p["solved"]
    tot = sum(v[0] for v in by.values()), sum(v[1] for v in by.values())
    for t, (s, n) in by.items():
        bar = "█" * int(20 * s / n) + "░" * (20 - int(20 * s / n))
        print(f"  {t:28} {bar} {s}/{n}")
    print(f"\n  TOTAL {tot[0]}/{tot[1]} ({100 * tot[0] / max(tot[1], 1):.0f}%)")


def exporter_docker(port):
    """Roda o exporter num container publicado em :port. Útil quando o firewall do host (ex.: ufw)
    bloqueia conexões container -> host: tráfego para porta publicada passa pelo FORWARD, não pelo INPUT."""
    ex = pathlib.Path(os.environ.get("PCA_EXAM_RESULTS", PROGRESS.parent / ".exam-results.json"))
    cmd = ["docker", "run", "-d", "--name", "pca-progress", "--restart", "unless-stopped",
           "-p", f"{port}:9199", "-v", f"{ROOT}:/challenges:ro",
           "-v", f"{PROGRESS.parent.resolve()}:/progress:ro", "-e", f"PCA_PROGRESS=/progress/{PROGRESS.name}",
           "-v", f"{ex.parent.resolve()}:/exam:ro", "-e", f"PCA_EXAM_RESULTS=/exam/{ex.name}",
           "python:3.14-alpine", "python", "/challenges/check.py", "exporter"]
    subprocess.run(["docker", "rm", "-f", "pca-progress"], capture_output=True)
    r = subprocess.run(cmd, capture_output=True, text=True)
    if r.returncode:
        sys.exit(r.stderr)
    print(f"container pca-progress expondo seu progresso em http://localhost:{port}/metrics\n"
          "parar: docker rm -f pca-progress")
    return 0


def cmd_exporter(ch, port):
    """Seu progresso vira métrica Prometheus: estudar Prometheus usando Prometheus."""
    class H(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            p = progress()
            lines = ["# HELP pca_challenges_total Desafios disponíveis.", "# TYPE pca_challenges_total gauge",
                     "# HELP pca_challenges_solved Desafios resolvidos.", "# TYPE pca_challenges_solved gauge",
                     "# HELP pca_challenge_attempts_total Tentativas de resposta.", "# TYPE pca_challenge_attempts_total counter",
                     "# HELP pca_challenge_hints_used Dicas usadas.", "# TYPE pca_challenge_hints_used gauge"]
            agg = {}
            for cid, c in ch.items():
                k = (c["topic"], c["level"])
                a = agg.setdefault(k, [0, 0, 0, 0])
                a[0] += 1
                a[1] += cid in p["solved"]
                a[2] += p["attempts"].get(cid, 0)
                a[3] += p["hints"].get(cid, 0)
            # cada família de métrica precisa vir agrupada (formato de exposição do Prometheus)
            fams = {name: [] for name in ("pca_challenges_total", "pca_challenges_solved",
                                          "pca_challenge_attempts_total", "pca_challenge_hints_used")}
            for (t, l), vals in sorted(agg.items()):
                lb = f'topic="{t}",level="{l}"'
                for name, v in zip(fams, vals):
                    fams[name].append(f"{name}{{{lb}}} {v}")
            out = []
            for i, name in enumerate(fams):
                out += lines[2 * i:2 * i + 2] + fams[name]
            lines = out
            ex = pathlib.Path(os.environ.get("PCA_EXAM_RESULTS", PROGRESS.parent / ".exam-results.json"))
            try:
                res = json.loads(ex.read_text())
            except Exception:
                res = {}
            if res.get("by_domain"):
                esc = lambda v: str(v).replace("\\", "\\\\").replace('"', '\\"')
                lines += ["# HELP pca_exam_score_ratio Nota do último simulado por domínio (0..1).",
                          "# TYPE pca_exam_score_ratio gauge"]
                for d, v in res["by_domain"].items():
                    lines.append(f'pca_exam_score_ratio{{domain="{esc(d)}"}} {float(v)}')
                if "score" in res:
                    lines += ["# HELP pca_exam_overall_score_ratio Nota geral do último simulado (0..1).",
                              "# TYPE pca_exam_overall_score_ratio gauge", f"pca_exam_overall_score_ratio {float(res['score'])}"]
                if "ts" in res:
                    lines += ["# HELP pca_exam_last_timestamp_seconds Quando o último simulado foi feito.",
                              "# TYPE pca_exam_last_timestamp_seconds gauge", f"pca_exam_last_timestamp_seconds {float(res['ts'])}"]
            body = ("\n".join(lines) + "\n").encode()
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; version=0.0.4")
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *a):
            pass
    print(f"expondo progresso em http://0.0.0.0:{port}/metrics (Ctrl+C para sair)", flush=True)
    http.server.ThreadingHTTPServer(("0.0.0.0", port), H).serve_forever()


def cmd_selftest(ch, only=()):
    """Para CI: cada gabarito roda, não vem vazio (salvo expect_empty) e bate consigo mesmo.
    Também roda `wrong_answers` (se houver) e exige que sejam REJEITADAS.
    `only`: ids ou tópicos para testar só uma parte (ex.: selftest counters 001)."""
    bad = 0
    if only:
        ch = {k: v for k, v in ch.items() if k in only or v["topic"] in only}
    for cid, c in ch.items():
        for k in ("id", "title", "topic", "level", "prompt", "solution", "explanation"):
            if not c.get(k):
                print(f"FAIL {cid}: campo '{k}' ausente"); bad += 1
        if c.get("level") not in ("basic", "intermediate", "advanced"):
            print(f"FAIL {cid}: level inválido {c.get('level')!r}"); bad += 1
        if not c.get("wrong_answers"):
            print(f"WARN {cid}: sem wrong_answers (recomendado ≥ 1)")
        ts = time.time() - EVAL_DELAY
        try:
            want = query(c["solution"], ts)
        except Exception as e:
            print(f"FAIL {cid}: gabarito não roda: {e}"); bad += 1; continue
        ok, why = compare(c, want, want)
        if not ok:
            print(f"FAIL {cid}: {why}"); bad += 1; continue
        for alt in c.get("accepted_answers", []):
            try:
                ok, why = compare(c, query(alt, ts), want)
            except Exception as e:
                ok, why = False, str(e)
            if not ok:
                print(f"FAIL {cid}: resposta alternativa aceita foi rejeitada: {alt} ({why})"); bad += 1
        for wrong in c.get("wrong_answers", []):
            try:
                ok, _ = compare(c, query(wrong, ts), want)
            except Exception:
                ok = False
            if ok:
                print(f"FAIL {cid}: resposta ERRADA foi aceita: {wrong}"); bad += 1
    print(f"selftest: {len(ch)} desafios, {bad} falhas")
    return 1 if bad else 0


def main():
    argv = sys.argv[1:]
    if not argv:
        print(__doc__); return 0
    cmd = argv[0]
    if cmd == "exporter":
        port = int(os.environ.get("PORT", 9199))
        if "--docker" in argv:
            return exporter_docker(port)
        cmd_exporter(load_meta(), port); return 0
    ch = load_all()
    if cmd == "list":
        ap = argparse.ArgumentParser(); ap.add_argument("--topic"); ap.add_argument("--level")
        cmd_list(ch, ap.parse_args(argv[1:])); return 0
    if cmd == "show": cmd_show(get(ch, argv[1])); return 0
    if cmd == "hint": cmd_hint(get(ch, argv[1])); return 0
    if cmd == "solution":
        c = get(ch, argv[1]); print(c["solution"]); print("\n" + c.get("explanation", "")); return 0
    if cmd == "progress": cmd_progress(ch); return 0
    if cmd == "selftest": return cmd_selftest(ch, set(argv[1:]))
    c = get(ch, cmd)
    if len(argv) >= 3 and argv[1] == "-f":
        ans = pathlib.Path(argv[2]).read_text()
    elif len(argv) >= 2:
        ans = argv[1]
    else:
        cmd_show(c); return 0
    return cmd_check(c, ans)


if __name__ == "__main__":
    sys.exit(main())
