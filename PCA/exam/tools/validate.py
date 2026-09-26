#!/usr/bin/env python3
"""Valida o banco de questões (questions/*.yaml). exit 1 se houver erro.

Checa: campos obrigatórios, domínio/dificuldade/idioma válidos, ids únicos, exatamente 4
alternativas não vazias e distintas, resposta A–D, enunciados não duplicados, fontes locais
existentes (aviso), mínimo de questões EN e cota do simulado coberta por domínio (EN).
"""
import collections, pathlib, re, sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from build import load, EXAM  # noqa: E402

PCA = EXAM.parent
DOMAINS = {"Observability Concepts": .18, "Prometheus Fundamentals": .20, "PromQL": .28,
           "Instrumentation and Exporters": .16, "Alerting and Dashboarding": .18}
DIFF = {"easy", "medium", "hard"}
MIN_EN = 150
SIMULADO_QUOTA = {"PromQL": 17, "Prometheus Fundamentals": 12, "Observability Concepts": 11,
                  "Alerting and Dashboarding": 11, "Instrumentation and Exporters": 9}


def norm(s):
    return re.sub(r"\s+", " ", str(s).lower()).strip()


def main():
    qs = load()
    errors, warns = [], []
    ids, texts, stems = {}, {}, {}
    for q in qs:
        where = f"{q['file']}:{q.get('id')}"
        for k in ["id", "domain", "difficulty", "question", "options", "answer", "explanation_pt", "source", "lang"]:
            if not q.get(k):
                errors.append(f"{where}: campo '{k}' ausente/vazio")
        if q.get("id") in ids:
            errors.append(f"{where}: id duplicado (também em {ids[q['id']]})")
        ids[q.get("id")] = q["file"]
        if q.get("domain") not in DOMAINS:
            errors.append(f"{where}: domínio inválido {q.get('domain')!r}")
        if q.get("difficulty") not in DIFF:
            errors.append(f"{where}: difficulty inválida {q.get('difficulty')!r}")
        if q.get("lang") not in ("en", "pt"):
            errors.append(f"{where}: lang inválido {q.get('lang')!r}")
        opts = q.get("options") or []
        if len(opts) != 4:
            errors.append(f"{where}: {len(opts)} alternativas (precisa 4)")
        elif any(not str(o).strip() for o in opts):
            errors.append(f"{where}: alternativa vazia")
        elif len({norm(o) for o in opts}) != 4:
            errors.append(f"{where}: alternativas repetidas")
        if q.get("answer") not in ("A", "B", "C", "D"):
            errors.append(f"{where}: answer {q.get('answer')!r} não é A–D")
        # duplicata = mesmo enunciado E mesmas alternativas; mesmo enunciado curto
        # ("Qual expressão é válida?") com alternativas diferentes é só aviso
        t = norm(q.get("question") or "")
        full = (t, tuple(sorted(norm(o) for o in opts)))
        if full in texts:
            errors.append(f"{where}: questão duplicada de {texts[full]}")
        elif t in stems:
            warns.append(f"{where}: mesmo enunciado de {stems[t]} (alternativas diferentes)")
        texts[full] = where
        stems.setdefault(t, where)
        src = q.get("source") or ""
        if src and not src.startswith("http"):
            if not (PCA / src.split("#")[0]).exists():
                warns.append(f"{where}: fonte local não existe (ainda?): {src}")

    by = collections.Counter((q.get("domain"), q.get("lang")) for q in qs)
    ans = collections.Counter(q.get("answer") for q in qs if q.get("lang") == "en")
    n_en = sum(1 for q in qs if q.get("lang") == "en")
    print(f"validate: {len(qs)} questões ({n_en} EN, {len(qs) - n_en} PT)")
    print(f"  {'domínio':32s} {'EN':>4s} {'PT':>4s} {'total':>6s}  cota-simulado")
    for d in DOMAINS:
        en, pt = by[(d, 'en')], by[(d, 'pt')]
        print(f"  {d:32s} {en:4d} {pt:4d} {en + pt:6d}  {SIMULADO_QUOTA[d]}")
        if en < SIMULADO_QUOTA[d]:
            errors.append(f"domínio {d}: só {en} questões EN, abaixo da cota do simulado ({SIMULADO_QUOTA[d]})")
    print(f"  gabarito EN: {dict(sorted(ans.items()))}")
    if n_en < MIN_EN:
        errors.append(f"só {n_en} questões EN (mínimo {MIN_EN})")
    for w in warns:
        print("  aviso: " + w)
    for e in errors:
        print("  ERRO: " + e)
    if errors:
        print(f"validate: FAIL ({len(errors)} erro(s))")
        sys.exit(1)
    print("validate: OK")


if __name__ == "__main__":
    main()
