# Convenções do material PCA

Tudo em `PCA/` segue estas regras, para o aluno ter uma experiência uniforme e para que **tudo seja testável automaticamente** (`./test-all.sh`).

## Versões fixas (iguais em todo lugar)

| Imagem | Tag |
|---|---|
| prom/prometheus | `v3.15.0` |
| grafana/grafana | `13.2.2` |
| prom/alertmanager | `v0.34.1` |
| prom/node-exporter | `v1.12.1` |
| prom/blackbox-exporter | `v0.28.0` |
| prom/pushgateway | `v1.11.3` |
| golang (build) | `1.27-alpine` |
| python | `3.14-alpine` |
| alpine (runtime de apps Go) | `3.23` |
| client_golang | `v1.24.1` |

`promtool` e `amtool` são usados **via docker** (`docker run --rm --entrypoint promtool prom/prometheus:v3.15.0 ...`), então o aluno não precisa instalar nada além de Docker.

## Mapa de portas (os labs podem rodar ao mesmo tempo)

| Lab | Portas no host |
|---|---|
| promql-functions-lab | Prometheus 9095 · Grafana 3300 · gerador 8088 |
| labs/alertmanager | Prometheus 9110 · Alertmanager 9111 · receiver 9112 · app 9113 |
| labs/service-discovery-relabeling | Prometheus 9120 · alvos 9121-9129 |
| labs/instrumentation | Prometheus 9130 · app-go 9131 · app-python 9132 |
| labs/exporters-pushgateway | Prometheus 9140 · node_exporter 9141 · blackbox 9142 · pushgateway 9143 |
| labs/tsdb-storage | Prometheus 9150 · gerador 9151 |
| labs/federation-remote-write | prom-a 9160 · prom-b 9161 · global 9162 · receiver 9163 · agent 9164 |
| labs/slo-end-to-end | Prometheus 9170 · Grafana 3170 · app 9171 |
| gamedays | Prometheus 9180 · Alertmanager 9181 · alvos 9182-9189 |
| challenges (progresso) | exporter de progresso 9199 |
| labs/recording-rules-testing | nenhuma (só promtool) |
| labs/promql-operators | usa o Prometheus do promql-functions-lab (9095) |

Cada compose tem `name: pca-<lab>` para não colidir com outros projetos.

## Estrutura de um lab (`labs/<lab>/`)

```
labs/<lab>/
├── README.md            # a aula (pt-BR), mesmo estilo das lições de promql-functions-lab
├── docker-compose.yml   # se precisar de stack
├── exercises/           # exercícios práticos "quebre e conserte" / "complete"
│   ├── 01-<nome>/README.md   # enunciado + dica + <details> com a solução
│   └── ...
├── solutions/           # arquivos de solução (usados pelo test.sh)
└── test.sh              # TESTE AUTOMÁTICO do lab: sobe, verifica, derruba. exit 0 = OK
```

README de lab: 🧠 Analogia · 🏗️ Arquitetura (diagrama ASCII ou mermaid) · ▶️ Como rodar · 🔍 Passo a passo · 🏭 Casos reais (com YAML real) · 🧪 Exercícios (links) · ⚠️ Pegadinhas · 🎓 Na prova PCA (≥ 5 perguntas estilo prova com `<details>`) · 📝 Cola rápida · 📚 Referências.

## `test.sh` (obrigatório)

- `set -euo pipefail`; sobe a stack (`docker compose up -d --build --wait` quando der), **espera por condição** (loops `until` com timeout, nunca `sleep` longo cego), verifica com `curl`/`promtool`/`amtool`, e no final `docker compose down -v`.
- Deve testar **as soluções dos exercícios** (ex.: aplica `solutions/` e verifica que o resultado esperado aparece), não só "a stack subiu".
- Imprime `PASS <lab>` ou `FAIL <lab>: motivo`. Tempo alvo: < 5 min.
- Aceita `KEEP=1` para não derrubar a stack no fim (útil para estudar).

## Idioma

Conteúdo em **pt-BR**. Termos técnicos em inglês quando é assim que aparecem na prova (`relabel_configs`, `group_wait`, `for`...). Questões de simulado: enunciado em **inglês** (como na prova real) e explicação em **pt-BR**.
