#!/usr/bin/env python3
"""Gera os flashcards da PCA (Anki .apkg + CSV + HTML offline).

Fontes (tudo é colhido de novo a cada execução, então é seguro rodar de novo
quando novos labs aparecerem):
  a) seções "📝 Cola rápida" e "⚠️ Pegadinhas" de
       promql-functions-lab/functions/*/README.md
       labs/*/README.md
  b) "Top 15 pegadinhas" de promql-functions-lab/PCA.md
  c) baralho curado à mão: flashcards/curated.yaml

Saídas (em flashcards/):
  dist/pca-flashcards.apkg   baralho Anki (subdecks por domínio, GUIDs estáveis)
  dist/flashcards.csv        CSV para o Anki (HTML, colunas guid/deck/tags)
  dist/quizlet.csv           CSV texto puro (termo,definição) p/ Quizlet & cia
  flashcards.html            página offline para estudo/impressão

Uso:
  python3 tools/build.py            # tudo (precisa de genanki para o .apkg)
  python3 tools/build.py --no-apkg  # só CSV + HTML (sem dependências além de PyYAML)
  python3 tools/build.py --stats    # imprime contagens por domínio/origem
"""
from __future__ import annotations

import argparse
import csv
import hashlib
import html
import json
import pathlib
import re
import sys

import yaml

HERE = pathlib.Path(__file__).resolve().parent.parent          # PCA/flashcards
PCA = HERE.parent                                               # PCA/
GITHUB = "https://github.com/apolzek/tryharding-certifications/blob/main/PCA/"
GITHUB_TREE = "https://github.com/apolzek/tryharding-certifications/tree/main/PCA/"

# Domínios oficiais da PCA (chave, nome, peso). A ordem define a numeração dos decks.
DOMAINS = [
    ("observability", "Observability Concepts", 18),
    ("fundamentals", "Prometheus Fundamentals", 20),
    ("promql", "PromQL", 28),
    ("instrumentation", "Instrumentation & Exporters", 16),
    ("alerting", "Alerting & Dashboarding", 18),
]
DOMAIN_NAME = {k: n for k, n, _ in DOMAINS}
DOMAIN_IDX = {k: i + 1 for i, (k, _, _) in enumerate(DOMAINS)}

# Lab -> domínio. Labs desconhecidos: tenta adivinhar pelo nome; senão "fundamentals".
LAB_DOMAIN = {
    "alertmanager": "alerting",
    "recording-rules-testing": "alerting",
    "service-discovery-relabeling": "fundamentals",
    "federation-remote-write": "fundamentals",
    "tsdb-storage": "fundamentals",
    "instrumentation": "instrumentation",
    "exporters-pushgateway": "instrumentation",
    "slo-end-to-end": "observability",
    "promql-operators": "promql",
}
LAB_GUESS = [
    (r"alert|rule|grafana|dashboard", "alerting"),
    (r"promql|operator|query", "promql"),
    (r"instrument|exporter|push|client", "instrumentation"),
    (r"slo|sli|observab|trace|log", "observability"),
]

ORIGINS = {
    "curated": "Curadas",
    "top15": "Top 15 pegadinhas",
    "lesson": "Lições PromQL",
    "lab": "Labs",
}

# IDs fixos: mudar isso quebra a atualização de quem já importou.
MODEL_BASIC_ID = 1607392319
MODEL_CLOZE_ID = 1607392320
GUID_NS = "pca-flashcards-v1"

COLA_RE = re.compile(r"^(#{2,4})\s+.*Cola r[aá]pida", re.I)
PEG_RE = re.compile(r"^(#{2,4})\s+.*Pegadinhas", re.I)
TOP15_RE = re.compile(r"^(#{2,4})\s+.*Top\s*15", re.I)
HEADING_RE = re.compile(r"^(#{1,6})\s+")
ITEM_RE = re.compile(r"^(\s*)(?:[-*+]|\d+[.)])\s+(.*)$")
# "(painel 6)", "(query 3)", "(queries 2 e 3)": referências internas da lição, sem sentido fora dela
LOCAL_REF_RE = re.compile(r"\s*\((?:painel|painéis|query|queries|passo)s?\s+[\d, e]+\)", re.I)
CLOZE_RE = re.compile(r"\{\{c(\d+)::(.*?)(?:::(.*?))?\}\}", re.S)


# ───────────────────────────── markdown mínimo ─────────────────────────────

def _split_code(text: str):
    """Divide em [(is_code, chunk)] respeitando `code` e ``code``."""
    out, i = [], 0
    for m in re.finditer(r"(`+)(.+?)\1", text):
        if m.start() > i:
            out.append((False, text[i:m.start()]))
        out.append((True, m.group(2).strip() if m.group(1) == "``" else m.group(2)))
        i = m.end()
    if i < len(text):
        out.append((False, text[i:]))
    return out


def resolve_link(href: str, base: pathlib.Path | None) -> str:
    """Converte link relativo de um README em caminho relativo a PCA/ (prefixo 'repo:')."""
    if re.match(r"^[a-z]+:", href) or href.startswith("#"):
        return href
    if base is None:
        return href
    target = (base / href).resolve()
    try:
        return "repo:" + str(target.relative_to(PCA)) + ("/" if href.endswith("/") else "")
    except ValueError:
        return href


def md_inline(text: str, base: pathlib.Path | None = None) -> str:
    """Markdown inline -> HTML (code, bold, italic, links). Links viram 'repo:...'.
    O código vira placeholder antes do negrito, então `**`a`/`b` texto**` funciona."""
    codes: list[str] = []
    buf = []
    for is_code, chunk in _split_code(text):
        if is_code:
            codes.append("<code>" + html.escape(chunk, quote=False) + "</code>")
            buf.append(f"\x00{len(codes) - 1}\x00")
        else:
            buf.append(html.escape(chunk, quote=False))
    s = "".join(buf)
    s = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", s)
    s = re.sub(r"(?<![\w*])\*(?!\s)([^*\x00]+?)(?<!\s)\*(?![\w*])", r"<i>\1</i>", s)

    def link(m):
        href = resolve_link(html.unescape(m.group(2)), base)
        return f'<a href="{html.escape(href)}">{m.group(1)}</a>'
    s = re.sub(r"\[([^\]]+)\]\(([^)\s]+)\)", link, s)
    return re.sub(r"\x00(\d+)\x00", lambda m: codes[int(m.group(1))], s)


def md_block(text: str, base: pathlib.Path | None = None) -> str:
    """Markdown simples em bloco: parágrafos, listas '-', blocos ```."""
    lines, out, i = text.strip("\n").split("\n"), [], 0
    while i < len(lines):
        ln = lines[i]
        if ln.strip().startswith("```"):
            buf, i = [], i + 1
            while i < len(lines) and not lines[i].strip().startswith("```"):
                buf.append(lines[i]); i += 1
            out.append("<pre><code>" + html.escape("\n".join(buf), quote=False) + "</code></pre>")
            i += 1
            continue
        if re.match(r"^\s*[-*]\s+", ln):
            items = []
            while i < len(lines) and re.match(r"^\s*[-*]\s+", lines[i]):
                items.append("<li>" + md_inline(re.sub(r"^\s*[-*]\s+", "", lines[i]), base) + "</li>")
                i += 1
            out.append("<ul>" + "".join(items) + "</ul>")
            continue
        if ln.strip():
            para = [ln.strip()]
            i += 1
            while i < len(lines) and lines[i].strip() and not re.match(r"^\s*([-*]\s+|```)", lines[i]):
                para.append(lines[i].strip()); i += 1
            out.append("<p>" + md_inline(" ".join(para), base) + "</p>")
            continue
        i += 1
    if len(out) == 1 and out[0].startswith("<p>"):
        return out[0][3:-4]
    return "".join(out)


def strip_html(s: str) -> str:
    s = re.sub(r"<br\s*/?>|</p>|</li>", "\n", s)
    s = re.sub(r"<li>", "• ", s)
    s = re.sub(r"<[^>]+>", "", s)
    return re.sub(r"\n{2,}", "\n", html.unescape(s)).strip()


def plain_md(text: str) -> str:
    """Remove marcações markdown (para comparar/medir)."""
    t = re.sub(r"\[([^\]]+)\]\([^)]+\)", r"\1", text)
    return t.replace("**", "").replace("`", "")


# ───────────────────────────── extração de seções ─────────────────────────────

def section_items(md: str, head_re: re.Pattern) -> list[str]:
    """Itens (bullets/numerados, com continuações) da seção cujo título casa head_re."""
    lines = md.split("\n")
    items: list[str] = []
    in_sec, level, fence, cur = False, 0, False, None
    for ln in lines:
        if not in_sec:
            m = head_re.match(ln)
            if m:
                in_sec, level = True, len(m.group(1))
            continue
        if ln.strip().startswith("```"):
            fence = not fence
            if cur is not None:
                items[cur] += "\n" + ln
            continue
        if fence:
            if cur is not None:
                items[cur] += "\n" + ln
            continue
        h = HEADING_RE.match(ln)
        if h and len(h.group(1)) <= level:
            break
        if ln.strip() == "---":
            continue
        m = ITEM_RE.match(ln)
        if m and len(m.group(1)) < 2:
            items.append(m.group(2).strip()); cur = len(items) - 1
        elif m and cur is not None:                     # sub-bullet: junta ao pai
            items[cur] += "; " + m.group(2).strip()
        elif ln.strip() and cur is not None and ln.startswith(("  ", "\t")):
            items[cur] += " " + ln.strip()
        elif not ln.strip():
            pass
        elif cur is not None and not ln.startswith(("|", ">")):
            items[cur] += " " + ln.strip()          # continuação "preguiçosa"
    return [re.sub(r"[ \t]+", " ", it).strip() for it in items if it.strip()]


# ───────────────────────────── bullet -> pergunta ─────────────────────────────

def _slow_find(text: str, token: str) -> int:
    in_code, i = False, 0
    while i < len(text):
        if text[i] == "`":
            in_code = not in_code
        elif not in_code and text.startswith(token, i):
            return i
        i += 1
    return -1


def _balanced(s: str) -> bool:
    return s.count("`") % 2 == 0 and s.count("**") % 2 == 0 and s.count("(") == s.count(")")


def _cloze_bold(text: str) -> str | None:
    n = [0]

    def rep(m):
        inner = m.group(1).replace("::", ":​:").replace("}}", "} }")
        n[0] += 1
        return "{{c1::" + inner + "}}"
    out = re.sub(r"\*\*(.+?)\*\*", rep, text)
    if not n[0]:
        return None
    hidden = sum(len(plain_md(m.group(1))) for m in re.finditer(r"\*\*(.+?)\*\*", text))
    if hidden > 0.75 * len(plain_md(text)):
        return None
    return out


def _cloze_code(text: str, ctx_fn: str | None) -> str | None:
    spans = [m for m in re.finditer(r"`([^`]+)`", text)]
    spans = [m for m in spans if not ctx_fn or m.group(1).split("(")[0] != ctx_fn]
    if not spans:
        return None
    m = max(spans, key=lambda m: len(m.group(1)))
    if len(m.group(1)) > 0.75 * len(plain_md(text)) or "}}" in m.group(1) or "::" in m.group(1):
        return None
    return text[:m.start()] + "{{c1::" + m.group(0) + "}}" + text[m.end():]


def bullet_to_card(text: str, ctx: str, ctx_fn: str | None, kind: str) -> dict:
    """Converte um bullet em card {type: basic|cloze, front/back | text}.

    Heurística (primeira que servir):
      pegadinha "**Título** explicação"   -> Q: título / A: explicação
      2+ negritos                         -> cloze escondendo os negritos
      "A → B"                             -> Q: "A → ?" / A: B
      "Pergunta? resposta"                -> Q/A
      1 negrito                           -> cloze
      "Rótulo: conteúdo"                  -> Q: "Rótulo: ?" / A: conteúdo
      "A = B"                             -> Q: "A = ?" / A: B
      código inline                       -> cloze no maior trecho de código
      senão                               -> "complete a frase"
    """
    t = text.strip()
    t = re.sub(r"^\d+[.)]\s+", "", t)

    if kind == "pegadinha":
        m = re.match(r"^\*\*(.+?)\*\*\s*[:.\-–—]?\s*(.*)$", t, re.S)
        if m and len(plain_md(m.group(2))) >= 8:
            title = m.group(1).rstrip(":. ")
            back = m.group(2).strip()
            if back[:1] in ",;(":            # o título é o começo da frase: mostra a frase inteira
                back = t
            return {"type": "basic", "front": f"⚠️ {ctx}: pegadinha «{title}». O que acontece / por quê?",
                    "back": back, "full": t}

    # "**Rótulo:** conteúdo" (estilo cola dos labs) -> Q: "Rótulo: ?"
    m = re.match(r"^\*\*([^*]{2,40}?):\*\*\s*(.+)$|^\*\*([^*]{2,40}?)\*\*:\s*(.+)$", t, re.S)
    if m:
        label, rest = (m.group(1), m.group(2)) if m.group(1) else (m.group(3), m.group(4))
        if len(plain_md(rest)) >= 4:
            return {"type": "basic", "front": f"{label}: ?", "back": rest.strip(), "full": t}

    bolds = re.findall(r"\*\*(.+?)\*\*", t)
    if len(bolds) >= 2:
        c = _cloze_bold(t)
        if c:
            return {"type": "cloze", "text": c, "full": t}

    for tok, fmt in (("→", "{} → ?"), ("?", "{}")):
        j = _slow_find(t, tok)
        if j > 2:
            left, right = t[:j].strip(), t[j + len(tok):].strip()
            if tok == "?":
                left += "?"
                if len(plain_md(left)) > 90 or left.count('"') % 2 or not t[j + 1:j + 2].isspace():
                    continue
            if len(plain_md(right)) >= 2 and _balanced(left) and _balanced(right):
                return {"type": "basic", "front": fmt.format(left), "back": right, "full": t}

    if len(bolds) == 1:
        c = _cloze_bold(t)
        if c:
            return {"type": "cloze", "text": c, "full": t}

    for tok in (": ", " = "):
        j = _slow_find(t, tok)
        if 2 < j and len(plain_md(t[:j])) <= 80:
            left, right = t[:j].strip(), t[j + len(tok):].strip()
            if len(plain_md(right)) >= 2 and _balanced(left) and _balanced(right):
                q = left + (": ?" if tok == ": " else " = ?")
                return {"type": "basic", "front": q, "back": right, "full": t}

    c = _cloze_code(t, ctx_fn)
    if c:
        return {"type": "cloze", "text": c, "full": t}

    words = t.split()
    k = max(3, len(words) // 2)
    head = " ".join(words[:k])
    if not _balanced(head):
        head = plain_md(head)
    return {"type": "basic", "front": f"Complete: {head} …", "back": t, "full": t}


# ───────────────────────────── coleta ─────────────────────────────

def guess_lab_domain(name: str) -> str:
    if name in LAB_DOMAIN:
        return LAB_DOMAIN[name]
    for rx, d in LAB_GUESS:
        if re.search(rx, name):
            return d
    return "fundamentals"


def harvest_readme(path: pathlib.Path, origin: str, name: str, domain: str, ctx: str, ctx_fn: str | None):
    md = path.read_text(encoding="utf-8")
    rel = str(path.parent.relative_to(PCA)) + "/"
    cards = []
    for kind, rx in (("cola", COLA_RE), ("pegadinha", PEG_RE)):
        for idx, item in enumerate(section_items(md, rx), 1):
            item = LOCAL_REF_RE.sub("", item)
            c = bullet_to_card(item, ctx, ctx_fn, kind)
            if c["type"] == "basic" and not c["front"].startswith("⚠️"):
                c["front"] = f"{ctx} · " + c["front"]
            elif c["type"] == "cloze":
                c["text"] = f"{ctx} · " + c["text"]
            c.update(
                id=f"{origin}:{name}:{kind}:{idx}",
                domain=domain, origin=origin, kind=kind,
                source=rel, base=path.parent,
                tags=[f"{origin}::{name}", f"kind::{kind}"],
                context=ctx,
            )
            cards.append(c)
    return cards


def harvest_lessons():
    cards = []
    for readme in sorted((PCA / "promql-functions-lab/functions").glob("*/README.md")):
        fn = readme.parent.name
        cards += harvest_readme(readme, "lesson", fn, "promql", f"`{fn}()`", fn)
    return cards


def harvest_labs():
    cards, labs = [], []
    for readme in sorted((PCA / "labs").glob("*/README.md")):
        lab = readme.parent.name
        labs.append(lab)
        cards += harvest_readme(readme, "lab", lab, guess_lab_domain(lab), f"lab {lab}", None)
    return cards, labs


def harvest_top15():
    path = PCA / "promql-functions-lab/PCA.md"
    if not path.exists():
        return []
    cards = []
    for idx, item in enumerate(section_items(path.read_text(encoding="utf-8"), TOP15_RE), 1):
        c = bullet_to_card(item, "Top 15", None, "top15")
        if c["type"] == "basic":
            c["front"] = "⚡ Top 15 · " + c["front"]
        c.update(id=f"top15:{idx}", domain="promql", origin="top15", kind="top15",
                 source="promql-functions-lab/PCA.md", base=path.parent,
                 tags=["top15", "kind::pegadinha"], context="Top 15 pegadinhas")
        cards.append(c)
    return cards


def load_curated():
    path = HERE / "curated.yaml"
    data = yaml.safe_load(path.read_text(encoding="utf-8")) or []
    cards, seen = [], set()
    for i, e in enumerate(data):
        cid = str(e.get("id") or "")
        if not cid:
            sys.exit(f"curated.yaml: card #{i} sem id")
        if cid in seen:
            sys.exit(f"curated.yaml: id duplicado {cid}")
        seen.add(cid)
        dom = e.get("domain")
        if dom not in DOMAIN_NAME:
            sys.exit(f"curated.yaml: {cid}: domain inválido {dom!r} (use {list(DOMAIN_NAME)})")
        if "cloze" in e:
            if not CLOZE_RE.search(e["cloze"]):
                sys.exit(f"curated.yaml: {cid}: cloze sem {{{{c1::...}}}}")
            c = {"type": "cloze", "text": e["cloze"].strip(), "full": ""}
            if e.get("extra"):
                c["extra"] = e["extra"]
        elif "front" in e and "back" in e:
            c = {"type": "basic", "front": str(e["front"]).strip(), "back": str(e["back"]).strip(), "full": ""}
        else:
            sys.exit(f"curated.yaml: {cid}: precisa de front+back ou cloze")
        src = e.get("source", "")
        if src and not re.match(r"^[a-z]+:", src):
            c["source"] = src
        else:
            c["source_url"] = src
            c["source"] = ""
        c.update(id=f"curated:{cid}", domain=dom, origin="curated", kind="curated",
                 base=None, tags=["curated"] + [str(t) for t in e.get("tags", [])], context="")
        cards.append(c)
    return cards


# ───────────────────────────── render ─────────────────────────────

def link_for(href: str, mode: str) -> str:
    """mode: 'html' (relativo a flashcards/) ou 'web' (GitHub)."""
    if not href.startswith("repo:"):
        return href
    p = href[5:]
    if mode == "html":
        return "../" + p
    return (GITHUB_TREE if p.endswith("/") else GITHUB) + p


def fix_links(h: str, mode: str) -> str:
    return re.sub(r'href="(repo:[^"]*)"', lambda m: f'href="{html.escape(link_for(html.unescape(m.group(1)), mode))}"', h)


def render(card: dict) -> dict:
    """Preenche campos HTML (front_html/back_html/text_html/extra_html/source_*)."""
    base = card.get("base")
    if card["type"] == "basic":
        card["front_html"] = md_block(card["front"], base)
        card["back_html"] = md_block(card["back"], base)
    else:
        # cloze: converte markdown mas preserva as marcas {{cN::...}}
        card["text_html"] = md_block(card["text"], base)
    extra = card.get("extra", "")
    if card.get("full") and card["type"] == "basic" and plain_md(card["back"]) != plain_md(card["full"]):
        extra = (extra + "\n\n" if extra else "") + "Cola completa: " + card["full"]
    card["extra_html"] = md_block(extra, base) if extra else ""
    if card.get("source"):
        card["source_path"] = card["source"]
        card["source_web"] = (GITHUB_TREE if card["source"].endswith("/") else GITHUB) + card["source"]
        card["source_html"] = "../" + card["source"]
    else:
        card["source_path"] = card.get("source_url", "")
        card["source_web"] = card["source_html"] = card.get("source_url", "")
    card["deck"] = deck_name(card)
    card["guid_key"] = f"{GUID_NS}|{card['id']}|{card['type']}"
    card["all_tags"] = [f"domain::{card['domain']}", f"origin::{card['origin']}"] + card["tags"]
    return card


def cloze_side(h: str, side: str) -> str:
    def rep(m):
        if side == "front":
            return f'<span class="cloze">[{m.group(3) or "…"}]</span>'
        return f'<span class="cloze-a">{m.group(2)}</span>'
    return CLOZE_RE.sub(rep, h)


def deck_name(card) -> str:
    d = card["domain"]
    return f"PCA::{DOMAIN_IDX[d]:02d} {DOMAIN_NAME[d]}::{ORIGINS[card['origin']]}"


def stable_int(s: str) -> int:
    return int(hashlib.sha1(s.encode()).hexdigest()[:12], 16) % (1 << 31) + (1 << 30)


def src_link(card, mode) -> str:
    url = card["source_web"] if mode == "web" else card["source_html"]
    if not url:
        return ""
    label = card["source_path"].rstrip("/") or url
    return f'<a href="{html.escape(url)}">{html.escape(label)}</a>'


# ───────────────────────────── saídas ─────────────────────────────

CARD_CSS = """
.card{font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif;font-size:19px;line-height:1.45;
 text-align:left;color:#1b1f24;background:#fbfbfa;max-width:760px;margin:0 auto;padding:6px 10px}
.night_mode .card,.nightMode .card{color:#e6e6e3;background:#1e1f22}
code{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:.88em;
 background:rgba(127,127,127,.16);padding:1px 4px;border-radius:4px}
pre{background:rgba(127,127,127,.12);padding:8px;border-radius:6px;overflow:auto;font-size:.85em}
pre code{background:none;padding:0}
.cloze{font-weight:700;color:#0a64d8}.night_mode .cloze,.nightMode .cloze{color:#6fb1ff}
.extra{margin-top:14px;font-size:.85em;opacity:.85}
.src{margin-top:14px;font-size:.72em;opacity:.7}
.dom{font-size:.7em;letter-spacing:.04em;text-transform:uppercase;opacity:.6;margin-bottom:8px}
hr#answer{margin:14px 0;opacity:.3}
"""


def build_apkg(cards, out: pathlib.Path):
    try:
        import genanki
    except ImportError:
        sys.exit("genanki não instalado: use tools/build.sh (cria .venv ou usa docker) ou --no-apkg")
    basic = genanki.Model(
        MODEL_BASIC_ID, "PCA Basic",
        fields=[{"name": "Front"}, {"name": "Back"}, {"name": "Extra"}, {"name": "Domain"}, {"name": "Source"}],
        templates=[{
            "name": "Card 1",
            "qfmt": '<div class="dom">{{Domain}}</div>{{Front}}',
            "afmt": '{{FrontSide}}<hr id="answer">{{Back}}{{#Extra}}<div class="extra">{{Extra}}</div>{{/Extra}}'
                    '<div class="src">{{Source}}</div>',
        }],
        css=CARD_CSS,
    )
    cloze = genanki.Model(
        MODEL_CLOZE_ID, "PCA Cloze",
        fields=[{"name": "Text"}, {"name": "Extra"}, {"name": "Domain"}, {"name": "Source"}],
        templates=[{
            "name": "Cloze",
            "qfmt": '<div class="dom">{{Domain}}</div>{{cloze:Text}}',
            "afmt": '<div class="dom">{{Domain}}</div>{{cloze:Text}}{{#Extra}}<div class="extra">{{Extra}}</div>{{/Extra}}'
                    '<div class="src">{{Source}}</div>',
        }],
        css=CARD_CSS,
        model_type=genanki.Model.CLOZE,
    )
    decks = {}
    for c in cards:
        name = c["deck"]
        if name not in decks:
            decks[name] = genanki.Deck(stable_int("deck|" + name), name)
        dom = DOMAIN_NAME[c["domain"]]
        tags = [re.sub(r"\s+", "_", t) for t in c["all_tags"]]
        if c["type"] == "basic":
            note = genanki.Note(model=basic, guid=genanki.guid_for(c["guid_key"]), tags=tags,
                                fields=[fix_links(c["front_html"], "web"), fix_links(c["back_html"], "web"),
                                        fix_links(c["extra_html"], "web"), dom, src_link(c, "web")])
        else:
            note = genanki.Note(model=cloze, guid=genanki.guid_for(c["guid_key"]), tags=tags,
                                fields=[fix_links(c["text_html"], "web"), fix_links(c["extra_html"], "web"),
                                        dom, src_link(c, "web")])
        decks[name].add_note(note)
    out.parent.mkdir(parents=True, exist_ok=True)
    pkg = genanki.Package([decks[k] for k in sorted(decks)])
    pkg.write_to_file(str(out))


def qa_html(c, mode):
    if c["type"] == "basic":
        return fix_links(c["front_html"], mode), fix_links(c["back_html"], mode)
    t = fix_links(c["text_html"], mode)
    return cloze_side(t, "front"), cloze_side(t, "back")


def build_csv(cards, out: pathlib.Path, quizlet: pathlib.Path):
    out.parent.mkdir(parents=True, exist_ok=True)
    with out.open("w", newline="", encoding="utf-8") as f:
        f.write("#separator:Comma\n#html:true\n#notetype:Basic\n#guid column:1\n#deck column:2\n#tags column:6\n")
        w = csv.writer(f)
        w.writerow(["guid", "deck", "front", "back", "source", "tags"])
        for c in cards:
            front, back = qa_html(c, "web")
            if c["extra_html"]:
                back += '<div class="extra">' + fix_links(c["extra_html"], "web") + "</div>"
            w.writerow([hashlib.sha1(("csv|" + c["guid_key"]).encode()).hexdigest()[:16],
                        c["deck"], front, back, c["source_web"],
                        " ".join(re.sub(r"\s+", "_", t) for t in c["all_tags"])])
    with quizlet.open("w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        for c in cards:
            front, back = qa_html(c, "web")
            w.writerow([strip_html(front), strip_html(back)])


def build_html(cards, out: pathlib.Path, template: pathlib.Path):
    data = []
    for c in cards:
        front, back = qa_html(c, "html")
        data.append({
            "id": c["id"], "d": c["domain"], "o": c["origin"], "k": c["kind"],
            "f": front, "b": back, "x": fix_links(c["extra_html"], "html"),
            "s": src_link(c, "html"),
        })
    meta = {
        "domains": [{"key": k, "name": n, "weight": w} for k, n, w in DOMAINS],
        "origins": ORIGINS,
    }
    page = template.read_text(encoding="utf-8")
    blob = json.dumps(data, ensure_ascii=False, separators=(",", ":")).replace("</", "<\\/")
    page = page.replace("/*__CARDS__*/[]", blob).replace("/*__META__*/{}", json.dumps(meta, ensure_ascii=False))
    page = page.replace("__COUNT__", str(len(data)))
    out.write_text(page, encoding="utf-8")


# ───────────────────────────── main ─────────────────────────────

def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--no-apkg", action="store_true", help="não gera o .apkg (dispensa genanki)")
    ap.add_argument("--stats", action="store_true", help="mostra contagens")
    ap.add_argument("--dump", metavar="ID_PREFIX", help="imprime os cards cujo id começa com o prefixo (debug)")
    a = ap.parse_args()

    lab_cards, labs = harvest_labs()
    cards = load_curated() + harvest_top15() + harvest_lessons() + lab_cards
    ids = [c["id"] for c in cards]
    dup = {i for i in ids if ids.count(i) > 1}
    if dup:
        sys.exit(f"ids duplicados: {sorted(dup)[:5]}")
    cards = [render(c) for c in cards]
    order = {k: i for i, (k, _, _) in enumerate(DOMAINS)}
    cards.sort(key=lambda c: (order[c["domain"]], list(ORIGINS).index(c["origin"])))

    if a.dump:
        for c in cards:
            if c["id"].startswith(a.dump):
                f, b = qa_html(c, "html")
                print(f"── {c['id']} [{c['type']}]\n  Q: {strip_html(f)}\n  A: {strip_html(b)}")
        return

    dist = HERE / "dist"
    build_csv(cards, dist / "flashcards.csv", dist / "quizlet.csv")
    build_html(cards, HERE / "flashcards.html", HERE / "tools/template.html")
    if not a.no_apkg:
        build_apkg(cards, dist / "pca-flashcards.apkg")

    by_dom = {k: 0 for k, _, _ in DOMAINS}
    by_org = {k: 0 for k in ORIGINS}
    for c in cards:
        by_dom[c["domain"]] += 1
        by_org[c["origin"]] += 1
    print(f"cards: {len(cards)} (basic {sum(c['type']=='basic' for c in cards)}, "
          f"cloze {sum(c['type']=='cloze' for c in cards)}) · labs colhidos: {', '.join(labs) or '(nenhum ainda)'}")
    if a.stats:
        for k, n, _ in DOMAINS:
            print(f"  {n:<30} {by_dom[k]}")
        for k, n in ORIGINS.items():
            print(f"  origem {n:<23} {by_org[k]}")
    (dist / "stats.json").write_text(json.dumps({"total": len(cards), "by_domain": by_dom, "by_origin": by_org,
                                                 "labs": labs}, indent=1))
    print("OK:", ", ".join(str(p.relative_to(HERE)) for p in
                           [HERE / "flashcards.html", dist / "flashcards.csv", dist / "quizlet.csv"] +
                           ([] if a.no_apkg else [dist / "pca-flashcards.apkg"])))


if __name__ == "__main__":
    main()
