#!/usr/bin/env python3
"""Gera o mapa mental interativo da PCA a partir de ../markmap.md.

Saídas (em mindmap/):
  markmap-links.md   cópia do markmap.md com links em cada nó (lição / lab / desafios / gameday / flashcards)
  index.html         markmap interativo (CDN jsDelivr, precisa de internet) + outline HTML puro (offline)

markmap.md NÃO é alterado. Rode de novo sempre que labs/desafios/gamedays novos aparecerem:
  python3 mindmap/build.py            # só biblioteca padrão do Python
  python3 mindmap/build.py --check    # também valida os links (igual ao test.sh)
"""
from __future__ import annotations

import argparse
import html
import json
import pathlib
import re
import sys

HERE = pathlib.Path(__file__).resolve().parent
PCA = HERE.parent
SRC = PCA / "markmap.md"
FUNCS = PCA / "promql-functions-lab/functions"
MARKMAP_VERSION = "0.18.12"      # markmap-autoloader / markmap-lib / markmap-view (npm, verificado em 2026-09)

# Domínio (título do "##") -> chave usada no flashcards.html (#d=...)
DOMAIN_KEYS = [
    (r"^PromQL", "promql"),
    (r"Fundamentals", "fundamentals"),
    (r"Observability", "observability"),
    (r"Alerting", "alerting"),
    (r"Instrumentation", "instrumentation"),
]

# Palavra-chave no texto do nó -> lab. (regex, pasta em labs/)
LAB_RULES = [
    (r"Alertmanager|routing tree|group_wait|group_interval|repeat_interval|inhibit|silence|receivers?|dedup|--cluster\.peer|Alerting Rules|`for`|pending|firing", "alertmanager"),
    (r"recording rules?|record:|promtool check rules|promtool test", "recording-rules-testing"),
    (r"^Scraping$|Service Discovery|_sd_configs|relabel|__meta_|__address__|__scheme__|__metrics_path__|static_configs|honor_labels|metric_relabel", "service-discovery-relabeling"),
    (r"remote_write|remote_read|federation|Thanos|Cortex|Mimir|HA needs|external_labels", "federation-remote-write"),
    (r"^Instrumentation$|client librar|client_golang|NewCounterVec|exposition|OpenMetrics|snake_case|low-cardinality|Metric Types|^Counter|^Gauge|^Histogram|^Summary|naming:|base units|`_total` suffix", "instrumentation"),
    (r"^Exporters$|node_exporter|blackbox|cAdvisor|mysqld_exporter|custom exporter|Pushgateway|push via|cron/batch|white-box|push \(Pushgateway\)", "exporters-pushgateway"),
    (r"Storage \(TSDB\)|TSDB|WAL|retention|compaction|blocks|head block", "tsdb-storage"),
    (r"SLI|SLO|SLA|error budget|RED method|USE method|Golden Signals|^Dashboarding$|Grafana", "slo-end-to-end"),
    (r"^Selectors$|^Operators$|label matchers|matchers:|range selector|offset|@ modifier|arithmetic|comparison|logical/set|vector matching|on\(label\)|group_left|Aggregation Operators|topk|count_values|grouping:|sum`, `min|stddev|Data Types|instant vector|range vector|scalar \(|string \(", "promql-operators"),
]

# Tópico de desafio -> regex de nós. Tópicos novos sem entrada aqui casam pelo próprio nome.
CHALLENGE_RULES = {
    "selectors-and-types": r"Data Types|^Selectors$|label matchers|^matchers:|range selector|offset|@ modifier|instant vector|range vector",
    "operators": r"^Operators$|arithmetic|comparison|logical/set|vector matching|group_left|on\(label\)",
    "aggregation": r"Aggregation Operators|topk|count_values|grouping:|`sum`",
    "counters": r"^`(rate|irate|increase)\(|Counters vs Gauges|counter reset|^Counter",
    "gauges": r"^`(delta|deriv|predict_linear)\(|gauges:|^Gauge",
    "histograms": r"histogram|^Histogram|^Summary",
    "over-time": r"_over_time",
    "absence-and-staleness": r"absent|staleness",
    "labels": r"label_replace|label_join|labels add dimensions",
    "labels-and-joins": r"label_replace|label_join|group_left",
    "subqueries": r"subquer",
    "time": r"offset modifier|@ modifier",
    "functions": r"^Functions$",
}

# Gameday (nome da pasta, sem o número) -> regex de nós. Novos sem entrada casam pelas palavras do nome.
GAMEDAY_RULES = {
    "alerta-que-nunca-dispara": r"Alerting Rules|`for`|pending",
    "alvo-sumiu": r"absent|Service Discovery|`up|target /metrics",
    "explosao-de-cardinalidade": r"cardinality|cardinalidade|low-cardinality",
    "counter-negativo": r"counter reset|Counters vs Gauges",
    "ninguem-foi-paginado": r"^Alertmanager$|routing tree",
    "backup-congelado": r"^Pushgateway$|cron/batch",
}

EXAM_RULES = r"^PCA$"


def slug_regex(name: str) -> str:
    words = [w for w in re.split(r"[-_]", name) if len(w) > 3]
    return "|".join(map(re.escape, words)) or re.escape(name)


def functions() -> set[str]:
    return {p.name for p in FUNCS.iterdir() if (p / "README.md").exists()} if FUNCS.exists() else set()


def discover(kind: str) -> list[str]:
    base = PCA / kind
    if not base.exists():
        return []
    return sorted(p.name for p in base.iterdir() if p.is_dir() and not p.name.startswith(("_", ".")) and p.name != "work")


def node_links(text: str, fns: set[str], heading_level: int | None) -> list[tuple[str, str, str]]:
    """Lista de (emoji+rótulo, href relativo a mindmap/, chave de dedupe)."""
    out: list[tuple[str, str, str]] = []
    plain = text.strip()
    # 1) funções PromQL citadas: `rate(...)`, rate(), `*_over_time()`
    seen = set()
    for m in re.finditer(r"\b([a-z_][a-z0-9_]*)\(", plain):
        fn = m.group(1)
        if fn in fns and fn not in seen:
            seen.add(fn)
            out.append((f"📘 {fn}", f"../promql-functions-lab/functions/{fn}/", f"fn:{fn}"))
    for m in re.finditer(r"`([a-z_][a-z0-9_]*)`", plain):
        fn = m.group(1)
        if fn in fns and fn not in seen:
            seen.add(fn)
            out.append((f"📘 {fn}", f"../promql-functions-lab/functions/{fn}/", f"fn:{fn}"))
    # 2) labs (existentes ou planejados)
    for rx, lab in LAB_RULES:
        if re.search(rx, plain):
            out.append((f"🧪 {lab}", f"../labs/{lab}/", f"lab:{lab}"))
    # 3) desafios
    for topic in discover("challenges"):
        rx = CHALLENGE_RULES.get(topic) or slug_regex(topic)
        if re.search(rx, plain, re.I if topic not in CHALLENGE_RULES else 0):
            out.append((f"🎯 {topic}", f"../challenges/{topic}/", f"ch:{topic}"))
    # 4) gamedays
    for gd in discover("gamedays"):
        key = re.sub(r"^\d+-", "", gd)
        rx = GAMEDAY_RULES.get(key) or slug_regex(key)
        if re.search(rx, plain, re.I if key not in GAMEDAY_RULES else 0):
            out.append((f"🔥 {key}", f"../gamedays/{gd}/", f"gd:{gd}"))
    return out


def domain_key(title: str) -> str | None:
    for rx, k in DOMAIN_KEYS:
        if re.search(rx, title):
            return k
    return None


def enrich(md: str) -> tuple[str, list[dict]]:
    """Devolve (markdown enriquecido, árvore para o outline)."""
    fns = functions()
    lines = md.split("\n")
    out, i = [], 0
    # frontmatter intacto (troca só o título)
    if lines and lines[0].strip() == "---":
        j = lines.index("---", 1)
        fm = lines[: j + 1]
        fm = [re.sub(r"^title:.*", "title: PCA · mapa mental com links", l) for l in fm]
        if "markmap:" in "\n".join(fm):          # mapa começa com os subtópicos visíveis e itens recolhidos
            k = next(n for n, l in enumerate(fm) if l.startswith("markmap:"))
            fm[k + 1:k + 1] = ["  initialExpandLevel: 3", "  maxWidth: 420"]
        out += fm
        i = j + 1
    stack: list[tuple[int, set[str]]] = []      # (profundidade, chaves de link já usadas nos ancestrais)
    tree: list[dict] = []
    tstack: list[tuple[int, dict]] = []
    for ln in lines[i:]:
        h = re.match(r"^(#{1,6})\s+(.*)$", ln)
        li = re.match(r"^(\s*)[-*]\s+(.*)$", ln)
        if not (h or li):
            out.append(ln)
            continue
        if h:
            depth, text, prefix = len(h.group(1)) - 1, h.group(2), h.group(1) + " "
        else:
            depth, text, prefix = 10 + len(li.group(1).replace("\t", "  ")) // 2, li.group(2), li.group(1) + "- "
        while stack and stack[-1][0] >= depth:
            stack.pop()
        used = set().union(*(s for _, s in stack)) if stack else set()
        links = [l for l in node_links(text, fns, len(h.group(1)) if h else None) if l[2] not in used]
        # extras por nível
        if h and depth == 0:
            links += [("🗓️ plano de estudo", "../STUDY-PLAN.md", "root:plan"), ("🃏 flashcards", "../flashcards/flashcards.html", "root:fc"),
                      ("🎯 desafios", "../challenges/", "root:ch"), ("📝 simulado", "../exam/", "root:exam"),
                      ("🔥 gamedays", "../gamedays/", "root:gd"), ("📚 funções PromQL", "../promql-functions-lab/", "root:lab")]
        if h and depth == 1 and domain_key(text):
            links.append(("🃏 cards", f"../flashcards/flashcards.html#d={domain_key(text)}", f"fc:{domain_key(text)}"))
        keys = {l[2] for l in links}
        stack.append((depth, keys))
        suffix = "".join(f" [{lab}]({href})" for lab, href, _ in links)
        out.append(prefix + text + suffix)
        node = {"text": text, "links": [(lab, href) for lab, href, _ in links], "children": [], "heading": bool(h)}
        while tstack and tstack[-1][0] >= depth:
            tstack.pop()
        (tstack[-1][1]["children"] if tstack else tree).append(node)
        tstack.append((depth, node))
    return "\n".join(out), tree


def md_inline(s: str) -> str:
    parts, i = [], 0
    for m in re.finditer(r"`([^`]+)`", s):
        parts.append(html.escape(s[i:m.start()])); parts.append("<code>" + html.escape(m.group(1)) + "</code>"); i = m.end()
    parts.append(html.escape(s[i:]))
    return re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", "".join(parts))


def outline_html(nodes: list[dict], depth=0) -> str:
    if not nodes:
        return ""
    items = []
    for n in nodes:
        links = " ".join(f'<a class="lk" href="{html.escape(h)}">{html.escape(l)}</a>' for l, h in n["links"])
        body = f'<span class="t">{md_inline(n["text"])}</span> {links}'
        if n["children"]:
            items.append(f"<li><details {'open' if depth < 2 else ''}><summary>{body}</summary>{outline_html(n['children'], depth + 1)}</details></li>")
        else:
            items.append(f"<li>{body}</li>")
    return f'<ul class="d{depth}">' + "".join(items) + "</ul>"


def all_links(md: str) -> list[str]:
    return [m.group(1) for m in re.finditer(r"\]\(([^)\s]+)\)", md)]


def check_links(md: str) -> tuple[list[str], list[str]]:
    """(erros, avisos). Link quebrado para conteúdo que já deveria existir = erro; para labs/desafios/etc. futuros = aviso."""
    errs, warns = [], []
    future = ("labs/", "challenges/", "exam/", "gamedays/", "flashcards/", "STUDY-PLAN.md")
    for href in sorted(set(all_links(md))):
        if re.match(r"^[a-z]+:", href):
            continue
        path = href.split("#")[0]
        target = (HERE / path).resolve()
        if target.exists():
            continue
        rel = str(target.relative_to(PCA)) if target.is_relative_to(PCA) else str(target)
        (warns if rel.startswith(future) else errs).append(rel)
    return errs, warns


PAGE = """<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>PCA Mapa Mental</title>
<meta name="description" content="Mapa mental interativo da PCA com links para lições, labs, desafios, gamedays e flashcards (gerado por mindmap/build.py).">
<style>
:root{--bg:#f6f5f2;--surface:#fff;--text:#1b1f24;--muted:#5d6570;--border:#dcdad3;--link:#0a64d8;--code:rgba(120,120,120,.14)}
@media (prefers-color-scheme: dark){:root:not([data-theme="light"]){--bg:#141517;--surface:#1d1f22;--text:#e7e6e2;--muted:#a2a7ae;--border:#34373c;--link:#6fb1ff;--code:rgba(200,200,200,.12)}}
:root[data-theme="dark"]{--bg:#141517;--surface:#1d1f22;--text:#e7e6e2;--muted:#a2a7ae;--border:#34373c;--link:#6fb1ff;--code:rgba(200,200,200,.12)}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
header{padding:14px 16px 6px;max-width:1200px;margin:0 auto}
header h1{margin:0;font-size:1.25rem}
header p{margin:2px 0 0;color:var(--muted);font-size:.88rem}
a{color:var(--link)}
code{font-family:ui-monospace,Menlo,Consolas,monospace;font-size:.88em;background:var(--code);padding:0 4px;border-radius:4px}
#map{margin:8px 16px;background:var(--surface);border:1px solid var(--border);border-radius:12px;height:78vh;position:relative;overflow:hidden}
#map .markmap{width:100%;height:100%}
#map svg{width:100%;height:100%;color:var(--text)}
#map .markmap-foreign a{color:var(--link)}
#offline{position:absolute;inset:0;display:none;align-items:center;justify-content:center;text-align:center;padding:20px;color:var(--muted)}
#outline{max-width:1200px;margin:18px auto 40px;padding:0 16px}
#outline h2{font-size:1.05rem}
#outline ul{list-style:none;padding-left:18px;margin:2px 0}
#outline ul.d0{padding-left:0}
#outline li{margin:2px 0}
#outline summary{cursor:pointer}
#outline .lk{font-size:.8rem;text-decoration:none;border:1px solid var(--border);border-radius:99px;padding:0 7px;margin-left:2px;white-space:nowrap;background:var(--surface)}
#outline .lk:hover{border-color:var(--link)}
.bar{display:flex;gap:8px;flex-wrap:wrap;margin-top:8px}
.bar a,.bar button{font:inherit;font-size:.85rem;border:1px solid var(--border);background:var(--surface);color:var(--text);border-radius:8px;padding:4px 10px;text-decoration:none;cursor:pointer}
@media print{#map,.bar{display:none}#outline details{display:block}#outline details>*{display:block}}
</style>
</head>
<body>
<header>
  <h1>PCA · Mapa mental</h1>
  <p>Clique nos nós para expandir ou recolher, e nos links para abrir a lição, o lab, os desafios ou os cards. O mapa usa markmap via CDN (precisa de internet); o <a href="#outline">outline abaixo</a> funciona offline.</p>
  <div class="bar"><a href="../STUDY-PLAN.md">🗓️ Plano de estudo</a><a href="../flashcards/flashcards.html">🃏 Flashcards</a><a href="markmap-links.md">📄 markmap-links.md</a><button id="expand" type="button">Expandir outline</button></div>
</header>
<div id="map">
  <div class="markmap"><script type="text/template">
__MD__
  </script></div>
  <div id="offline">Não consegui carregar o markmap (jsDelivr). Sem internet? Use o <a href="#outline">outline</a> logo abaixo: ele tem os mesmos nós e links.</div>
</div>
<section id="outline">
  <h2>Outline (HTML puro, funciona offline)</h2>
  __OUTLINE__
</section>
<script>
window.markmap = { autoLoader: { manual: false, toolbar: true } };
setTimeout(function(){
  if (!document.querySelector("#map svg g")) document.getElementById("offline").style.display = "flex";
}, 6000);
document.getElementById("expand").onclick = function(){
  document.querySelectorAll("#outline details").forEach(function(d){ d.open = true; });
  location.hash = "outline";
};
</script>
<script src="https://cdn.jsdelivr.net/npm/markmap-autoloader@__VER__"></script>
</body>
</html>
"""


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--check", action="store_true", help="valida links (erro só para conteúdo que já deveria existir)")
    a = ap.parse_args()
    md, tree = enrich(SRC.read_text(encoding="utf-8"))
    (HERE / "markmap-links.md").write_text(md + ("\n" if not md.endswith("\n") else ""), encoding="utf-8")
    body = md.split("---", 2)[2] if md.startswith("---") else md
    fm = "---\n" + md.split("---", 2)[1].strip() + "\n---\n" if md.startswith("---") else ""
    page = (PAGE.replace("__MD__", (fm + body.strip()).replace("</script", "<\\/script"))
                .replace("__OUTLINE__", outline_html(tree)).replace("__VER__", MARKMAP_VERSION))
    (HERE / "index.html").write_text(page, encoding="utf-8")
    n_links = len(all_links(md))
    print(f"mindmap: {n_links} links em {sum(1 for l in md.splitlines() if '](' in l)} nós · labs={discover('labs')} "
          f"· desafios={discover('challenges')} · gamedays={discover('gamedays')}")
    if a.check:
        errs, warns = check_links(md)
        for w in warns:
            print(f"  WARN link ainda sem destino (será criado por outro módulo): {w}")
        for e in errs:
            print(f"  ERRO link quebrado: {e}")
        if errs:
            sys.exit(1)


if __name__ == "__main__":
    main()
