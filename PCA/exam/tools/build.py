#!/usr/bin/env python3
"""Junta questions/*.yaml em questions.json e questions.js (este último permite abrir o
index.html direto via file://, sem servidor, pois é carregado com <script src>)."""
import json, pathlib, sys

import yaml

EXAM = pathlib.Path(__file__).resolve().parent.parent
FIELDS = ["id", "domain", "difficulty", "question", "options", "answer", "explanation_pt", "source", "lang"]


def load():
    qs = []
    for f in sorted((EXAM / "questions").glob("*.y*ml")):
        data = yaml.safe_load(f.read_text(encoding="utf-8")) or []
        if not isinstance(data, list):
            sys.exit(f"{f}: esperado uma lista de questões")
        for q in data:
            q = {k: q.get(k) for k in FIELDS} | {"file": f.name}
            if isinstance(q.get("options"), list):
                q["options"] = [str(o) for o in q["options"]]
            qs.append(q)
    return qs


def main():
    qs = load()
    (EXAM / "questions.json").write_text(json.dumps(qs, ensure_ascii=False, indent=0), encoding="utf-8")
    js = ("// GERADO por tools/build.py — não edite. Fonte: questions/*.yaml\n"
          "window.PCA_QUESTIONS = " + json.dumps(qs, ensure_ascii=False) + ";\n")
    (EXAM / "questions.js").write_text(js, encoding="utf-8")
    print(f"build: {len(qs)} questões -> questions.js / questions.json")


if __name__ == "__main__":
    main()
