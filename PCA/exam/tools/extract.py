#!/usr/bin/env python3
"""Extrai os quizzes "🎓 Na prova PCA" dos READMEs para o banco de questões do simulado.

Fontes:
  promql-functions-lab/functions/*/README.md  -> questions/extracted-functions.yaml
  labs/*/README.md                             -> questions/extracted-labs.yaml

Formato esperado (tolerante a variações):
  **1.** Enunciado (pode continuar em várias linhas / blocos de código)
  - A) opção            (ou tudo numa linha: "- A) x  B) y  C) z  D) w")
  - B) ...
  <details><summary>Resposta</summary>
  **B.** justificativa...      (também aceita "**B**", "**B)**", "**B** →")
  </details>

É idempotente: pode rodar quantas vezes quiser (os arquivos gerados são sobrescritos).
No final imprime o que NÃO conseguiu interpretar (exit 0 mesmo assim; use --strict para exit 1).
"""
import argparse, pathlib, re, sys

import yaml

EXAM = pathlib.Path(__file__).resolve().parent.parent
PCA = EXAM.parent
QDIR = EXAM / "questions"

DOMAINS = ["Observability Concepts", "Prometheus Fundamentals", "PromQL",
           "Instrumentation and Exporters", "Alerting and Dashboarding"]

# lab -> domínio (labs novos caem no fallback com aviso)
LAB_DOMAIN = {
    "alertmanager": "Alerting and Dashboarding",
    "recording-rules-testing": "Alerting and Dashboarding",
    "service-discovery-relabeling": "Prometheus Fundamentals",
    "tsdb-storage": "Prometheus Fundamentals",
    "federation-remote-write": "Prometheus Fundamentals",
    "instrumentation": "Instrumentation and Exporters",
    "exporters-pushgateway": "Instrumentation and Exporters",
    "slo-end-to-end": "Observability Concepts",
    "promql-operators": "PromQL",
}

# Perguntas das funções são PromQL, a menos que o ENUNCIADO seja claramente de outro domínio.
KW_ALERT = re.compile(r"alertmanager|grafana|painel|dashboard|recording rule|keep_firing_for", re.I)
KW_INSTR = re.compile(r"client librar|biblioteca cliente|instrumentad|exporter|pushgateway|tipo de métrica|metric type|openmetrics", re.I)
KW_QUERY = re.compile(r"\b(qual|que|escreva|como fica a)\s+(expressão|query|consulta)\b", re.I)

SECTION_RE = re.compile(r"^##\s+.*Na prova PCA", re.I)
QSTART_RE = re.compile(r"^\*\*(\d+)[.)]\*\*\s*(.*)$")
QSTART2_RE = re.compile(r"^\*\*(\d+)[.)]\s*(.+?)\*\*\s*(.*)$")  # **1. Enunciado em negrito**
OPT_LINE_RE = re.compile(r"^\s*[-*]?\s*\(?([A-Da-d])\)\s+(.*)$")
INLINE_OPTS_RE = re.compile(r"(?:^|\s)\(?([A-D])\)\s+")
ANSWER_RE = re.compile(r"^\s*\*\*\s*([A-D])\s*[.):]?\s*\*\*\s*[.):→-]*\s*(.*)$")
ANSWER_ALT_RE = re.compile(r"(?:Resposta|Gabarito|Answer)\s*[:：]?\s*\**\s*([A-D])\b", re.I)


def section_lines(text):
    out, on = [], False
    for ln in text.splitlines():
        if SECTION_RE.match(ln):
            on = True
            continue
        if on and re.match(r"^##\s", ln):
            break
        if on:
            out.append(ln)
    return out


def split_inline(s):
    """'A) x  B) y  C) z  D) w' -> [x, y, z, w] (ou None)."""
    marks = list(INLINE_OPTS_RE.finditer(s))
    letters = [m.group(1) for m in marks]
    if letters[:4] != ["A", "B", "C", "D"] or len(marks) != 4:
        return None
    opts = []
    for i, m in enumerate(marks):
        end = marks[i + 1].start() if i + 1 < len(marks) else len(s)
        opts.append(s[m.end():end].strip())
    return opts


def parse_block(lines):
    """lines = linhas de UMA questão (do **N.** até o próximo **M.**). Retorna dict ou raise ValueError."""
    first = lines[0]
    m = QSTART_RE.match(first)
    if m:
        qtext = [m.group(2)]
    else:
        m = QSTART2_RE.match(first)
        qtext = [m.group(2) + (" " + m.group(3) if m.group(3) else "")]
    opts, i, in_code = {}, 1, False
    # enunciado até a primeira opção (fora de bloco de código)
    while i < len(lines):
        ln = lines[i]
        if ln.strip().startswith("```"):
            in_code = not in_code
        if not in_code:
            inl = split_inline(ln.strip().lstrip("-* ").strip()) if re.match(r"^\s*[-*]?\s*\(?A\)", ln) else None
            if inl:
                opts = dict(zip("ABCD", inl))
                i += 1
                break
            om = OPT_LINE_RE.match(ln)
            if om and om.group(1).upper() == "A":
                break
            if "<details" in ln:
                break
        qtext.append(ln)
        i += 1
    if not opts:
        while i < len(lines):
            ln = lines[i]
            om = OPT_LINE_RE.match(ln)
            if om and om.group(1).upper() in "ABCD" and om.group(1).upper() not in opts:
                opts[om.group(1).upper()] = om.group(2).strip()
            elif "<details" in ln:
                break
            elif ln.strip() and opts and not om:
                # continuação de opção multilinha
                last = sorted(opts)[-1]
                opts[last] += " " + ln.strip()
            i += 1
    # resposta
    det = []
    while i < len(lines) and "<details" not in lines[i]:
        i += 1
    i += 1
    while i < len(lines) and "</details>" not in lines[i]:
        det.append(lines[i])
        i += 1
    answer, expl = None, []
    for ln in det:
        if answer is None:
            am = ANSWER_RE.match(ln)
            if am:
                answer = am.group(1)
                if am.group(2).strip():
                    expl.append(am.group(2).strip())
                continue
            am = ANSWER_ALT_RE.search(ln)
            if am:
                answer = am.group(1).upper()
                rest = ln[am.end():].strip(" *.:→-")
                if rest:
                    expl.append(rest)
                continue
            if not ln.strip():
                continue
        expl.append(ln)
    question = "\n".join(qtext).strip()
    if sorted(opts) != ["A", "B", "C", "D"]:
        raise ValueError(f"opções encontradas: {sorted(opts) or 'nenhuma'}")
    if not answer:
        raise ValueError("resposta (**X.**) não encontrada no <details>")
    if not question:
        raise ValueError("enunciado vazio")
    return {"question": question, "options": [opts[k] for k in "ABCD"], "answer": answer,
            "explanation_pt": "\n".join(expl).strip() or "(sem justificativa no README)"}


def split_questions(sec):
    blocks, cur = [], None
    in_code = False
    for ln in sec:
        if ln.strip().startswith("```"):
            in_code = not in_code
        if not in_code and (QSTART_RE.match(ln) or QSTART2_RE.match(ln)):
            if cur:
                blocks.append(cur)
            cur = [ln]
        elif cur is not None:
            cur.append(ln)
    if cur:
        blocks.append(cur)
    return blocks


PT_WORDS = re.compile(r"\b(os|as|que|de|do|da|é|um|uma|qual|quais|não|com|para|em|por|se|quando|como)\b|ção|ções|ã", re.I)
EN_WORDS = re.compile(r"\b(the|is|are|what|which|of|does|do|when|how|with|for|an|to|in|and|should|you|about|that|this|was|if|before|after)\b", re.I)


def detect_lang(text):
    """Os quizzes são em pt-BR, mas alguns labs escrevem o enunciado em inglês (estilo prova).
    Enunciado claramente em inglês -> lang: en (entra no filtro "só inglês")."""
    text = re.sub(r"`[^`]*`|```.*?```", " ", text, flags=re.S)
    return "en" if len(EN_WORDS.findall(text)) > 2 * len(PT_WORDS.findall(text)) else "pt"


def classify_function(q):
    text = q["question"]
    if KW_QUERY.search(text):
        return "PromQL"
    if KW_ALERT.search(text):
        return "Alerting and Dashboarding"
    if KW_INSTR.search(text):
        return "Instrumentation and Exporters"
    return "PromQL"


def extract(files, kind, problems):
    out = []
    for f in files:
        name = f.parent.name
        sec = section_lines(f.read_text(encoding="utf-8"))
        rel = f.relative_to(PCA).as_posix()
        if not sec:
            problems.append(f"{rel}: seção '🎓 Na prova PCA' não encontrada")
            continue
        blocks = split_questions(sec)
        if not blocks:
            problems.append(f"{rel}: seção existe mas nenhuma questão '**N.**' encontrada")
            continue
        seen = set()
        for b in blocks:
            num = (QSTART_RE.match(b[0]) or QSTART2_RE.match(b[0])).group(1)
            try:
                q = parse_block(b)
            except ValueError as e:
                problems.append(f"{rel} questão {num}: {e}")
                continue
            prefix = "fn" if kind == "functions" else "lab"
            qid = f"{prefix}-{name}-{num}"
            if qid in seen:
                problems.append(f"{rel}: número de questão repetido {num}")
                continue
            seen.add(qid)
            if kind == "functions":
                domain = classify_function(q)
            else:
                domain = LAB_DOMAIN.get(name)
                if not domain:
                    domain = "Prometheus Fundamentals"
                    problems.append(f"{rel}: lab '{name}' sem domínio mapeado em LAB_DOMAIN (usando {domain})")
            out.append({"id": qid, "domain": domain, "difficulty": "medium", **q,
                        "source": f.parent.relative_to(PCA).as_posix() + "/README.md",
                        "lang": detect_lang(q["question"])})
    return out


def dump(path, qs, header):
    class D(yaml.SafeDumper):
        pass

    def str_rep(d, s):
        return d.represent_scalar("tag:yaml.org,2002:str", s, style="|" if "\n" in s else None)
    D.add_representer(str, str_rep)
    body = yaml.dump(qs, Dumper=D, allow_unicode=True, sort_keys=False, width=1000)
    path.write_text(header + body, encoding="utf-8")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--strict", action="store_true", help="exit 1 se algo não pôde ser interpretado")
    a = ap.parse_args()
    problems = []
    QDIR.mkdir(exist_ok=True)
    hdr = "# GERADO por tools/extract.py (não edite à mão; edite o README de origem e rode de novo)\n"
    fn = extract(sorted(PCA.glob("promql-functions-lab/functions/*/README.md")), "functions", problems)
    labs = extract(sorted(PCA.glob("labs/*/README.md")), "labs", problems)
    pending = [d.name for d in sorted(PCA.glob("labs/*/")) if d.is_dir() and not (d / "README.md").exists()]
    dump(QDIR / "extracted-functions.yaml", fn, hdr)
    dump(QDIR / "extracted-labs.yaml", labs, hdr)
    print(f"extract: {len(fn)} questões de functions, {len(labs)} de labs")
    by = {}
    for q in fn + labs:
        by[q["domain"]] = by.get(q["domain"], 0) + 1
    for d in DOMAINS:
        print(f"  {d:32s} {by.get(d, 0)}")
    if pending:
        print(f"extract: labs ainda sem README.md (rode de novo depois): {', '.join(pending)}")
    if problems:
        print(f"extract: {len(problems)} problema(s) — não importados:")
        for p in problems:
            print("  ! " + p)
    if problems and a.strict:
        sys.exit(1)


if __name__ == "__main__":
    main()
