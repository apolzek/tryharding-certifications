# 🎓 Simulado PCA (Fase 3)

Simulado da **Prometheus Certified Associate** no formato da prova: **60 questões de múltipla escolha**, **90 minutos**, enunciados em **inglês** e explicações em **português**, sorteadas nos **pesos oficiais** dos domínios.

| Domínio | Peso | Questões no simulado |
|---|---|---|
| PromQL | 28% | 17 |
| Prometheus Fundamentals | 20% | 12 |
| Observability Concepts | 18% | 11 |
| Alerting and Dashboarding | 18% | 11 |
| Instrumentation and Exporters | 16% | 9 |

> As cotas saem do método do maior resto sobre 60 (16,8 → 17; 10,8 → 11; 9,6 → 9). A nota de referência é **75%**. A Linux Foundation não divulga a nota de corte exata, então use o número como meta, não como garantia.

## ▶️ Como usar

### No navegador (sem servidor, funciona offline)

Abra `exam/index.html` direto no navegador (`file://` funciona, porque as questões vêm de `questions.js`).

- **Simulado**: 60 questões nos pesos oficiais, cronômetro de 90 min e nenhum feedback até o fim. Ao final aparecem a nota geral, a nota por domínio (a linha vertical marca 75%) e a revisão com explicação e link para a lição/lab/doc de cada questão. As alternativas são **embaralhadas** durante a prova; na revisão elas voltam à ordem original, para bater com as explicações que citam letras.
- **Treino**: escolha um domínio e receba feedback + explicação a cada resposta.
- **Idioma**: "Só inglês" (como a prova real) ou "Todas" (inclui as questões em português extraídas das lições do `promql-functions-lab`).
- **Histórico** fica no `localStorage` do navegador. O botão **Baixar resultado (JSON)** salva nota, notas por domínio e cada resposta.
- Atalhos: `1`–`4` / `A`–`D` para responder, `←` `→` para navegar. Tema claro/escuro no botão "Tema".

### No terminal

```bash
cd exam
./exam.py simulado                 # 60 questões, 90 min, correção no fim (Enter pula; volta às puladas no fim)
./exam.py simulado --lang all      # inclui as questões em PT
./exam.py treino --domain promql   # feedback imediato (aceita parte do nome: alert, fund, instr, obs)
./exam.py treino --domain alert --n 30
./exam.py stats                    # quantas questões há por domínio/idioma
```

O resultado do **simulado** é gravado em `challenges/.exam-results.json`:

```json
{"by_domain": {"PromQL": 0.82, "Prometheus Fundamentals": 0.75, ...}, "score": 0.78, "ts": 1790000000, "history": [...]}
```

Esse arquivo alimenta o exporter de progresso dos desafios (métrica `pca_exam_score_ratio{domain}`). Use `--results outro.json` (ou `$PCA_EXAM_RESULTS`) para gravar em outro lugar, ou `--no-save` para não gravar. No treino o resultado só é salvo com `--save`.

Modo não interativo (usado nos testes):

```bash
./exam.py simulado --seed 42 --answers respostas.json   # {"pq-001": "A", "obs-007": "B", ...}
./exam.py simulado --seed 42 --answers respostas.txt    # linhas "pq-001 A"
```

As letras são as da **ordem original** do banco. Com a mesma `--seed`, `exam.py` e o `sampler.js` sorteiam **exatamente as mesmas questões** (mesmo RNG, e o teste confere).

## 🗂️ Banco de questões

```
exam/
├── questions/
│   ├── observability.yaml        # questões originais (EN) ─┐
│   ├── fundamentals.yaml         #                          │ escritas para o simulado,
│   ├── promql.yaml               # operadores/agregação/    │ explicação em pt-BR
│   │                             # vector matching          │
│   ├── instrumentation.yaml      #                          │
│   ├── alerting.yaml             #                         ─┘
│   ├── extracted-functions.yaml  # GERADO: quizzes "🎓 Na prova PCA" do promql-functions-lab
│   └── extracted-labs.yaml       # GERADO: quizzes "🎓 Na prova PCA" de labs/*
├── tools/
│   ├── extract.py    # README.md -> extracted-*.yaml (reexecutável; lista o que não conseguiu ler)
│   ├── build.py      # questions/*.yaml -> questions.js + questions.json
│   └── validate.py   # schema, ids únicos, 4 alternativas, resposta A–D, duplicatas, contagem por domínio
├── sampler.js        # sorteio com pesos (browser + node)
├── index.html        # simulador web
├── exam.py           # simulador de terminal (réplica do sampler em Python)
└── test.sh
```

Formato de cada questão:

```yaml
- id: fund-004                      # único
  domain: Prometheus Fundamentals   # um dos 5 domínios
  difficulty: easy                  # easy | medium | hard
  question: |
    If scrape_interval is not set anywhere in the configuration, what value does Prometheus use?
  options: ['15s', '30s', '5m', '1m']   # exatamente 4 (A–D)
  answer: D
  explanation_pt: |
    O default global é `scrape_interval: 1m` ...
  source: https://prometheus.io/docs/...   # URL ou caminho relativo a PCA/ (ex.: labs/alertmanager/README.md)
  lang: en                          # en | pt
```

Markdown simples funciona no enunciado, nas alternativas e na explicação: `` `código` ``, `**negrito**` e blocos ```` ``` ````.

### Extração dos quizzes das lições e labs

`tools/extract.py` lê a seção `## 🎓 Na prova PCA` de `promql-functions-lab/functions/*/README.md` e de `labs/*/README.md`. Ele espera questões `**N.**`, alternativas `- A) ...` (ou todas numa linha `- A) x  B) y  C) z  D) w`) e a resposta dentro de `<details>` como `**B.** justificativa`.

- **Domínio**: as funções caem em *PromQL*, a não ser que o enunciado seja claramente de alerting/dashboard (Alertmanager, Grafana, painel, recording rule) ou de instrumentação (client library, exporter, Pushgateway, tipo de métrica). Os labs usam o mapa `LAB_DOMAIN` (lab novo sem mapeamento cai em *Prometheus Fundamentals*, com aviso).
- **Idioma**: os quizzes são em pt-BR (`lang: pt`). Se o enunciado estiver claramente em inglês, como em alguns labs, a questão recebe `lang: en` e entra no filtro "só inglês".
- O que não puder ser interpretado é **listado** no fim (arquivo e número da questão) e fica fora do banco. `--strict` faz o script sair com erro nesses casos.

Depois de novos labs/quizzes, rode de novo:

```bash
cd exam && python3 tools/extract.py && python3 tools/build.py && python3 tools/validate.py
# ou simplesmente: ./test.sh   (faz os três e mais os testes)
```

Os arquivos `extracted-*.yaml`, `questions.js` e `questions.json` são gerados. **Não edite à mão**: corrija o README de origem ou o YAML original.

### Escrevendo questões novas

- Enunciado em **inglês**, direto, com **uma** resposta claramente correta (sem pegadinha de ambiguidade).
- Explicação em **pt-BR**: por que a certa é certa e por que as outras estão erradas. Evite citar letras ("A) ..."), porque o treino embaralha as alternativas quando a explicação não cita letras.
- Distribua o gabarito entre A–D. O `validate.py` mostra a distribuição.
- Valores técnicos conferidos contra Prometheus **v3.15** / Alertmanager **v0.34** (ex.: `scrape_interval` default 1m, `scrape_timeout` 10s, retenção 15d, `group_wait` 30s, `group_interval` 5m, `repeat_interval` 4h).

## 🧪 Teste

```bash
./test.sh          # ~2 s
SHOT=1 ./test.sh   # também salva /tmp/pca-exam.png
```

O teste:

1. roda `extract.py`, `build.py` e `validate.py` (falha em erro de schema, id duplicado, alternativas ≠ 4, resposta fora de A–D, questão duplicada, menos de 150 questões EN ou domínio sem questões EN suficientes para a cota do simulado);
2. sorteia 100 simulados × 2 idiomas com o **mesmo `sampler.js` do navegador** (via `node`) e confere 17/12/11/11/9, a ausência de ids repetidos e a ausência de questões PT no modo EN;
3. confere que a réplica Python (`exam.py sample`) sorteia os mesmos ids que o JS;
4. roda `exam.py` sem interação com respostas 100% certas, com o domínio Observability todo errado, e em treino, verificando o JSON `{by_domain, score, ts}`;
5. abre o `index.html#selftest` no **chromium headless**, que faz dois simulados completos (EN e todas), finaliza, gera a revisão, responde um treino e publica o resultado no DOM. O teste exige 0 erros de JS e cotas corretas.
