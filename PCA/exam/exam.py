#!/usr/bin/env python3
"""Simulado PCA no terminal.

  ./exam.py simulado                 # 60 questões nos pesos oficiais, 90 min, correção no fim
  ./exam.py simulado --lang all      # inclui as questões em PT extraídas das lições
  ./exam.py treino --domain PromQL   # feedback imediato (domínios: veja ./exam.py stats)
  ./exam.py stats                    # questões por domínio/idioma e cotas do simulado
  ./exam.py sample --seed 42         # (teste) mostra os ids sorteados — igual ao sampler.js com a mesma seed

Não interativo (CI / testes):
  ./exam.py simulado --seed 42 --answers respostas.json [--results /tmp/r.json]
  respostas.json = {"<id>": "B", ...} (letra ORIGINAL do banco; ids fora do sorteio são ignorados)
  ou um texto com linhas "<id> <letra>".

O resultado do simulado vai para ../challenges/.exam-results.json (ou --results / $PCA_EXAM_RESULTS):
  {"by_domain": {"PromQL": 0.8, ...}, "score": 0.77, "ts": 1790000000, ...}
que alimenta o exporter de progresso (métrica pca_exam_score_ratio{domain}).
"""
import argparse, json, os, pathlib, re, subprocess, sys, textwrap, time

EXAM = pathlib.Path(__file__).resolve().parent
RESULTS = pathlib.Path(os.environ.get("PCA_EXAM_RESULTS", EXAM.parent / "challenges" / ".exam-results.json"))

# ---- réplica do sampler.js (mesmos pesos, mesmo RNG mulberry32, mesmo Fisher-Yates) ----
WEIGHTS = [("PromQL", 0.28), ("Prometheus Fundamentals", 0.20), ("Observability Concepts", 0.18),
           ("Alerting and Dashboarding", 0.18), ("Instrumentation and Exporters", 0.16)]
DOMAINS = [d for d, _ in WEIGHTS]
PASS_MARK, EXAM_MINUTES = 0.75, 90
M = 0xFFFFFFFF


def rng(seed):
    a = [(seed & M) or 1]

    def imul(x, y):
        return (x * y) & M

    def nxt():
        a[0] = (a[0] + 0x6D2B79F5) & M
        t = a[0]
        t = imul(t ^ (t >> 15), t | 1)
        t ^= (t + imul(t ^ (t >> 7), t | 61)) & M
        t &= M
        return ((t ^ (t >> 14)) & M) / 4294967296
    return nxt


def shuffle(arr, rand):
    a = list(arr)
    for i in range(len(a) - 1, 0, -1):
        j = int(rand() * (i + 1))
        a[i], a[j] = a[j], a[i]
    return a


def quotas(n):
    q, rest, used = {}, [], 0
    for i, (d, w) in enumerate(WEIGHTS):
        exact = n * w
        q[d] = int(exact // 1)
        used += q[d]
        rest.append((exact - q[d], i, d))
    rest.sort(key=lambda r: (-r[0], r[1]))
    k = 0
    while used < n:
        q[rest[k % len(rest)][2]] += 1
        used += 1
        k += 1
    return q


def filter_lang(qs, lang):
    return [q for q in qs if lang in (None, "all") or q["lang"] == lang]


def draw(questions, n=60, lang="all", rand=None):
    import random
    rand = rand or random.random
    pool = filter_lang(questions, lang)
    by = {d: shuffle([q for q in pool if q["domain"] == d], rand) for d in DOMAINS}
    q, out, shortfall = quotas(n), [], 0
    for d in DOMAINS:
        take = min(q[d], len(by[d]))
        shortfall += q[d] - take
        out += by[d][:take]
        by[d] = by[d][take:]
    missing = shortfall
    for d in DOMAINS:
        while missing > 0 and by[d]:
            out.append(by[d].pop(0))
            missing -= 1
    return shuffle(out, rand), q, shortfall


LETTER_REF = re.compile(r"(^|[^\w`$])[A-D](\)|\s+e\s+[A-D]\b|,\s*[A-D]\b|\s)")


def cites_letters(q):
    """Mesmo critério do sampler.js: explicação cita letras -> não embaralhar no treino."""
    return bool(LETTER_REF.search(q.get("explanation_pt") or ""))


def score(questions, answers):
    detail = {d: {"correct": 0, "total": 0} for d in DOMAINS}
    ok = 0
    for q in questions:
        b = detail.setdefault(q["domain"], {"correct": 0, "total": 0})
        b["total"] += 1
        if answers.get(q["id"]) == q["answer"]:
            b["correct"] += 1
            ok += 1
    ratios = {d: b["correct"] / b["total"] for d, b in detail.items() if b["total"]}
    return {"correct": ok, "total": len(questions), "score": ok / len(questions) if questions else 0.0,
            "by_domain": ratios, "detail": detail}


# ---- carga ----
def load():
    j = EXAM / "questions.json"
    srcs = list((EXAM / "questions").glob("*.y*ml"))
    if not j.exists() or any(s.stat().st_mtime > j.stat().st_mtime for s in srcs):
        subprocess.run([sys.executable, str(EXAM / "tools" / "build.py")], check=True, stdout=subprocess.DEVNULL)
    return json.loads(j.read_text(encoding="utf-8"))


# ---- terminal ----
C = {"g": "\033[32m", "r": "\033[31m", "y": "\033[33m", "b": "\033[1m", "d": "\033[2m", "c": "\033[36m", "x": "\033[0m"}
if not sys.stdout.isatty():
    C = {k: "" for k in C}


def fmt(s):
    s = re.sub(r"```[a-zA-Z]*\n?(.*?)```", lambda m: C["c"] + m.group(1).rstrip() + C["x"], s, flags=re.S)
    s = re.sub(r"`([^`]+)`", lambda m: C["c"] + m.group(1) + C["x"], s)
    return re.sub(r"\*\*([^*]+)\*\*", lambda m: C["b"] + m.group(1) + C["x"], s)


def wrap(s, indent=""):
    out = []
    for para in str(s).split("\n"):
        out.append(textwrap.fill(para, 100, initial_indent=indent, subsequent_indent=indent) if para.strip() else "")
    return "\n".join(out)


def show_q(i, n, q, order, extra=""):
    print(f"\n{C['d']}── {i}/{n} · {q['domain']} · {q['difficulty']} · {q['lang'].upper()} {extra}{C['x']}")
    print(fmt(wrap(q["question"])))
    for pos, orig in enumerate(order):
        print(f"  {C['b']}{'ABCD'[pos]}){C['x']} {fmt(q['options'][orig])}")


def ask(prompt):
    try:
        return input(prompt).strip().upper()
    except EOFError:
        return "Q"


def explain(q, given):
    ok = given == q["answer"]
    mark = f"{C['g']}✔ correto{C['x']}" if ok else f"{C['r']}✘ errado{C['x']}"
    print(f"{mark} — resposta {C['b']}{q['answer']}{C['x']} (ordem original)" + ("" if ok or not given else f", você: {given}"))
    for k, o in enumerate(q["options"]):
        print(f"    {'ABCD'[k]}) {fmt(o)}")
    print(fmt(wrap(q["explanation_pt"], "  ")))
    print(f"  {C['d']}fonte: {q['source']}{C['x']}")


def read_answers(path):
    txt = pathlib.Path(path).read_text(encoding="utf-8")
    try:
        data = json.loads(txt)
        if isinstance(data, list):  # [{"id":..,"answer":..}]
            data = {d["id"]: d.get("given") or d.get("answer") for d in data}
    except json.JSONDecodeError:
        data = dict(ln.split()[:2] for ln in txt.splitlines() if ln.strip() and not ln.startswith("#"))
    return {k: str(v).upper() for k, v in data.items() if v}


def report(qs, answers, mode, lang, started, results_path=None, save=True, verbose=True):
    sc = score(qs, answers)
    passed = sc["score"] >= PASS_MARK
    col = C["g"] if passed else C["r"]
    print(f"\n{C['b']}Resultado ({mode}): {col}{sc['score'] * 100:.1f}%{C['x']} {sc['correct']}/{sc['total']}"
          f"  {'(≥75%: bom sinal)' if passed else '(abaixo da referência de 75%)'}")
    for d in DOMAINS:
        b = sc["detail"].get(d)
        if b and b["total"]:
            r = b["correct"] / b["total"]
            bar = "█" * round(r * 20) + "·" * (20 - round(r * 20))
            print(f"  {d:32s} {bar} {r * 100:5.1f}% ({b['correct']}/{b['total']})")
    if verbose:
        wrong = [q for q in qs if answers.get(q["id"]) != q["answer"]]
        if wrong and sys.stdin.isatty() and ask(f"\nRevisar os {len(wrong)} erros? [s/N] ") == "S":
            for q in wrong:
                print(f"\n{C['d']}{q['id']} · {q['domain']}{C['x']}")
                print(fmt(wrap(q["question"])))
                explain(q, answers.get(q["id"]))
    out = {"by_domain": sc["by_domain"], "score": sc["score"], "ts": int(time.time()), "mode": mode, "lang": lang,
           "correct": sc["correct"], "total": sc["total"], "duration_s": int(time.time() - started)}
    if save:
        path = pathlib.Path(results_path) if results_path else RESULTS
        prev = {}
        try:
            prev = json.loads(path.read_text())
        except Exception:
            pass
        out["history"] = (prev.get("history") or [])[-19:] + [{k: out[k] for k in ("ts", "mode", "lang", "score", "correct", "total")}]
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(out, indent=1, ensure_ascii=False))
        print(f"{C['d']}resultado salvo em {path}{C['x']}")
    return out


def cmd_simulado(a, qs_all):
    import random
    rand = rng(a.seed) if a.seed is not None else random.random
    qs, _, shortfall = draw(qs_all, a.n, a.lang, rand)
    if shortfall:
        print(f"{C['y']}aviso: faltaram {shortfall} questões em algum domínio; completei com outros.{C['x']}")
    started = time.time()
    if a.answers:
        answers = read_answers(a.answers)
        return report(qs, answers, "simulado", a.lang, started, a.results, not a.no_save, verbose=False)
    deadline = started + EXAM_MINUTES * 60
    answers, orders = {}, {q["id"]: shuffle(range(4), rand) for q in qs}
    print(f"{C['b']}Simulado PCA{C['x']}: {len(qs)} questões, {EXAM_MINUTES} min. Enter pula, q encerra, número volta (ex.: #12).")
    pending = list(range(len(qs)))
    while pending:
        skipped = []
        for i in pending:
            left = deadline - time.time()
            if left <= 0:
                print(f"{C['r']}Tempo esgotado!{C['x']}")
                pending = []
                break
            q = qs[i]
            show_q(i + 1, len(qs), q, orders[q["id"]], f"· ⏱ {int(left // 60)}:{int(left % 60):02d}")
            r = ask("Resposta [A-D, Enter=pular, q=finalizar]: ")
            if r == "Q":
                pending = []
                break
            if r in ("A", "B", "C", "D"):
                answers[q["id"]] = "ABCD"[orders[q["id"]]["ABCD".index(r)]]
            else:
                skipped.append(i)
        else:
            if skipped and ask(f"\n{len(skipped)} puladas. Voltar a elas? [S/n] ") != "N":
                pending = skipped
                continue
            pending = []
    return report(qs, answers, "simulado", a.lang, started, a.results, not a.no_save)


def cmd_treino(a, qs_all):
    import random
    rand = rng(a.seed) if a.seed is not None else random.random
    doms = [d for d in DOMAINS if a.domain.lower() in d.lower()] if a.domain else DOMAINS
    if not doms:
        sys.exit(f"domínio '{a.domain}' não encontrado. Opções: {', '.join(DOMAINS)}")
    qs = shuffle([q for q in filter_lang(qs_all, a.lang) if q["domain"] in doms], rand)[: a.n]
    started, answers = time.time(), {}
    if a.answers:
        answers = read_answers(a.answers)
        return report(qs, answers, "treino", a.lang, started, a.results, a.save, verbose=False)
    print(f"{C['b']}Treino{C['x']}: {', '.join(doms)} — {len(qs)} questões. q encerra.")
    done = []
    for i, q in enumerate(qs, 1):
        order = [0, 1, 2, 3] if cites_letters(q) else shuffle(range(4), rand)
        show_q(i, len(qs), q, order)
        r = ask("Resposta [A-D, q=encerrar]: ")
        if r == "Q":
            break
        if r not in ("A", "B", "C", "D"):
            continue
        answers[q["id"]] = "ABCD"[order["ABCD".index(r)]]
        done.append(q)
        explain(q, answers[q["id"]])
    if done:
        return report(done, answers, "treino", a.lang, started, a.results, a.save, verbose=False)


def cmd_stats(a, qs):
    print(f"{len(qs)} questões")
    for d in DOMAINS:
        en = sum(1 for q in qs if q["domain"] == d and q["lang"] == "en")
        pt = sum(1 for q in qs if q["domain"] == d and q["lang"] == "pt")
        print(f"  {d:32s} EN {en:4d}  PT {pt:4d}")
    print("cotas do simulado (60):", quotas(60))


def cmd_sample(a, qs):
    got, q, short = draw(qs, a.n, a.lang, rng(a.seed))
    print(json.dumps({"ids": [x["id"] for x in got], "quotas": q, "shortfall": short}))


def main():
    ap = argparse.ArgumentParser(description="Simulado PCA no terminal", formatter_class=argparse.RawDescriptionHelpFormatter, epilog=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)
    for name in ("simulado", "treino", "sample"):
        p = sub.add_parser(name)
        p.add_argument("--lang", choices=["en", "all", "pt"], default="en")
        p.add_argument("--seed", type=int)
        p.add_argument("--n", type=int, default=60 if name != "treino" else 20)
        if name != "sample":
            p.add_argument("--answers", help="arquivo de respostas (modo não interativo)")
            p.add_argument("--results", help=f"onde salvar o resultado (default {RESULTS})")
        if name == "simulado":
            p.add_argument("--no-save", action="store_true")
        if name == "treino":
            p.add_argument("--domain", help="parte do nome do domínio, ex.: promql, alert, fund")
            p.add_argument("--save", action="store_true", help="salvar o resultado do treino também")
    sub.add_parser("stats")
    a = ap.parse_args()
    if a.cmd == "sample" and a.seed is None:
        a.seed = 1
    qs = load()
    {"simulado": cmd_simulado, "treino": cmd_treino, "stats": cmd_stats, "sample": cmd_sample}[a.cmd](a, qs)


if __name__ == "__main__":
    main()
