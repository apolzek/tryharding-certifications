# 🗺️ Mapa mental interativo da PCA

- **[`index.html`](index.html)**: abra no navegador. Mostra o mapa do [`markmap.md`](../markmap.md) com links em cada nó:
  📘 lição da função · 🧪 lab · 🎯 tópico de desafios · 🔥 gameday · 🃏 cards do domínio.
  - O mapa usa **markmap** (`markmap-autoloader@0.18.12`) via **CDN jsDelivr**, então **precisa de internet**.
  - Sem internet, use o **outline em HTML puro** logo abaixo do mapa: tem os mesmos nós e links e também serve para imprimir.
- **[`markmap-links.md`](markmap-links.md)**: cópia enriquecida do `markmap.md`, com os links. Abre na extensão *Markmap* do VS Code ou em https://markmap.js.org/repl.
  O `markmap.md` original **não é alterado**.

## Gerar de novo

Rode de novo quando chegarem labs, tópicos de desafio ou gamedays novos. Os links são descobertos a partir das pastas existentes:

```bash
python3 mindmap/build.py          # só biblioteca padrão
mindmap/test.sh                   # gera + valida links (+ render no chromium, se houver)
```

Regras de link ficam no topo de [`build.py`](build.py) (`LAB_RULES`, `CHALLENGE_RULES`, `GAMEDAY_RULES`).
Um link para lab/desafio/gameday que ainda não existe gera **aviso**. Um link quebrado para conteúdo que já deveria existir
(ex.: `promql-functions-lab`) gera **erro**.
