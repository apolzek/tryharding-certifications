#!/usr/bin/env python3
"""Gera o índice de funções no README.md raiz (entre os marcadores INDEX)."""
import pathlib, re, collections, yaml, json
ROOT = pathlib.Path(__file__).resolve().parent.parent
groups = collections.defaultdict(list)
for labf in sorted((ROOT / "functions").glob("*/lab.yaml")):
    lab = yaml.safe_load(labf.read_text())
    fn = labf.parent.name
    st = ROOT / ".lab-status" / f"{fn}.json"
    ok = "✅" if st.exists() and json.loads(st.read_text()).get("notified") else "⏳"
    title = lab["title"].split("—", 1)[-1].strip() if "—" in lab["title"] else lab["title"]
    groups[lab.get("category", "99 - Outros")].append(
        f"| {ok} | [`{fn}()`](functions/{fn}/) | {title} | [dashboard](http://localhost:3300/d/fn-{fn}) |")
out = []
total = sum(len(v) for v in groups.values())
out.append(f"_{total} funções. ✅ = testada e validada._\n")
for cat in sorted(groups):
    out.append(f"### {cat}\n\n| | Função | O que ensina | Grafana |\n|---|---|---|---|")
    out.extend(groups[cat]); out.append("")
readme = ROOT / "README.md"
s = readme.read_text()
s = re.sub(r"<!-- INDEX:START -->.*<!-- INDEX:END -->",
           "<!-- INDEX:START -->\n" + "\n".join(out) + "\n<!-- INDEX:END -->", s, flags=re.S)
readme.write_text(s)
print(f"índice: {total} funções")
