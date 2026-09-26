# 🃏 Flashcards PCA (Anki + CSV + página offline)

Mais de **1.000 cards** para revisar a PCA com repetição espaçada, em três formatos:

| Arquivo | Para quê |
|---|---|
| [`dist/pca-flashcards.apkg`](dist/pca-flashcards.apkg) | baralho **Anki** pronto (subdecks por domínio, cards cloze e pergunta/resposta) |
| [`dist/flashcards.csv`](dist/flashcards.csv) | CSV para **importar no Anki** (HTML, com colunas deck/tags/GUID) |
| [`dist/quizlet.csv`](dist/quizlet.csv) | CSV **texto puro** `termo,definição` (Quizlet, Knowt, RemNote, Mochi...) |
| [`flashcards.html`](flashcards.html) | página **offline**: cards que viram, filtro por domínio, embaralhar, atalhos de teclado e modo lista/impressão |

## De onde vêm os cards

| Origem | Qtde (aprox.) | Como é gerado |
|---|---|---|
| **Curadas** ([`curated.yaml`](curated.yaml)) | 150+ | escritas à mão, alto rendimento, cobrindo os **5 domínios** |
| **Top 15 pegadinhas** | 15 | a lista de [`PCA.md`](../promql-functions-lab/PCA.md) virou cloze |
| **Lições PromQL** | ~900 | seções **📝 Cola rápida** e **⚠️ Pegadinhas** de cada [`functions/*/README.md`](../promql-functions-lab/functions/) |
| **Labs** | cresce sozinho | as mesmas seções de cada [`labs/*/README.md`](../labs/). Rode o build de novo quando um lab novo chegar |

Cada bullet vira uma pergunta de verdade: `A → B` vira "A → ?", `**negrito**` vira lacuna (cloze),
`Rótulo: conteúdo` vira "Rótulo: ?" e cada pegadinha vira "pegadinha «título»: o que acontece?". O verso sempre traz
o link para a lição de origem.

Os decks ficam assim (os domínios são numerados na ordem da página da prova):

```
PCA::01 Observability Concepts::{Curadas, Labs}
PCA::02 Prometheus Fundamentals::{Curadas, Labs}
PCA::03 PromQL::{Curadas, Top 15 pegadinhas, Lições PromQL, Labs}
PCA::04 Instrumentation & Exporters::{Curadas, Labs}
PCA::05 Alerting & Dashboarding::{Curadas, Labs}
```

Tags: `domain::promql`, `origin::curated|lesson|lab|top15`, `lesson::rate`, `lab::alertmanager`, `kind::cola|pegadinha`.
Dá para montar *filtered decks* no Anki com elas (ex.: `tag:kind::pegadinha`).

## 📥 Importar no Anki

1. Instale o [Anki](https://apps.ankiweb.net/) (desktop). No celular: AnkiDroid (grátis) ou AnkiMobile. Sincronize pelo AnkiWeb.
2. **Arquivo → Importar...** → escolha `dist/pca-flashcards.apkg`. Pronto: aparece o deck `PCA` com os subdecks.
3. **Atualizar depois** (lab novo, correção): gere de novo e importe o mesmo `.apkg`. As notas têm **GUID estável**,
   então o Anki **atualiza** as existentes em vez de duplicar, e o seu histórico de revisão é mantido.

> Use **ou** o `.apkg` **ou** o CSV, não os dois: são notas diferentes para o Anki.

Via CSV (alternativa): **Arquivo → Importar** → `dist/flashcards.csv`. O cabeçalho do arquivo já informa separador,
HTML, colunas de deck/tags/GUID e o tipo de nota *Basic*. Os cloze viram pergunta/resposta com `[…]`.

## 🗓️ Rotina sugerida (repetição espaçada)

Configuração do deck `PCA` (engrenagem → *Options*):

| Opção | Valor | Por quê |
|---|---|---|
| New cards/day | **40** (semanas 1-3) · **0** na semana da prova | ~1.100 cards ÷ 40 ≈ 4 semanas; o que sobrar das *Lições PromQL* fica para depois da prova |
| Maximum reviews/day | 300 | não deixe acumular |
| Learning steps | `10m 1d` | |
| FSRS | **ligado**, desired retention **0.90** | agenda melhor que o SM-2 clássico |
| Ordem | *Curadas* → *Top 15* → *Labs* → *Lições PromQL* | o que mais cai primeiro |

**Todo dia (15-25 min), de preferência na mesma hora:**

- [ ] Zere as **revisões** antes dos cards novos.
- [ ] Faça os **novos** do subdeck da semana (veja o [STUDY-PLAN](../STUDY-PLAN.md)).
- [ ] Errou 3× o mesmo card? Abra o link do verso e releia a lição; se o card estiver ruim, corrija a fonte (README/`curated.yaml`) e gere de novo.
- [ ] Seja honesto: **Again** quando não lembrou, **Good** quando lembrou. Evite o *Easy* no começo.

**Semana da prova:** zero cards novos. Só revisões + `tag:kind::pegadinha` num *filtered deck* + a página HTML em modo
"ocultar conhecidas" para uma varredura final.

## 🌐 Página offline (`flashcards.html`)

Abra no navegador (duplo clique; funciona sem internet). Os cards marcados como **conhecidos** ficam salvos no `localStorage`
do navegador (se o navegador bloquear o armazenamento, a página funciona do mesmo jeito, só não lembra).

| Tecla | Ação |
|---|---|
| `espaço` / `enter` | virar |
| `→` `l` / `←` `h` | próxima / anterior |
| `k` | marcar como conhecida (e avançar) |
| `s` | embaralhar |
| `u` | ocultar/mostrar conhecidas |
| `0`–`5` | todos / domínio 1..5 |
| `/` | buscar |
| `v` | modo lista (bom para **imprimir**: `Ctrl+P` imprime a lista filtrada, pergunta + resposta) |
| `t` | tema claro/escuro |

Links diretos: `flashcards.html#d=alerting`, `#q=group_wait`, `#o=curated&list`.

## 🔧 Gerar de novo / adicionar cards

```bash
cd PCA/flashcards
tools/build.sh            # gera tudo (cria .venv com genanki==0.13.1; sem venv, usa docker python:3.14-alpine)
USE_DOCKER=1 tools/build.sh
python3 tools/build.py --no-apkg      # só HTML + CSV (precisa só de PyYAML)
python3 tools/build.py --dump curated:alert-0   # ver como ficaram alguns cards
./test.sh                 # build + valida .apkg (zip + sqlite + nº de notas), CSV e HTML (chromium headless)
```

- **Card novo curado:** adicione um bloco em [`curated.yaml`](curated.yaml) com `id` **novo e estável** (o id vira o GUID no Anki; renomear = nota nova).
- **Lições e labs:** não precisa fazer nada. Tudo que estiver em `📝 Cola rápida` e `⚠️ Pegadinhas` entra sozinho no próximo build.
  Os cards desses READMEs são identificados por *arquivo + seção + posição*: editar o texto de um bullet **atualiza** a nota.
  Inserir um bullet no meio da lista desloca os seguintes (o conteúdo é atualizado, mas o histórico de revisão fica com a posição).
- Labs novos caem no domínio certo por nome (tabela `LAB_DOMAIN` em [`tools/build.py`](tools/build.py)).
