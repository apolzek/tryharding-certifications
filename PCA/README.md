# Prometheus Certified Associate (PCA)

Material **prático** para a PCA: você não só lê, você **faz** e é **corrigido automaticamente**.
Tudo roda em Docker com versões fixas (Prometheus v3.15.0, Grafana 13.2.2, Alertmanager v0.34.1…). Veja [CONVENTIONS.md](CONVENTIONS.md).

> 🚀 **Por onde começar:** siga a [**trilha de 4 semanas (STUDY-PLAN.md)**](STUDY-PLAN.md). Ela amarra lição → desafio → lab → game day → simulado, dia a dia.

## 🧰 O que tem aqui

| | O que é | Domínios da prova |
|---|---|---|
| 🔥 [**promql-functions-lab**](promql-functions-lab/) | As **90 funções PromQL**, uma pasta cada: aula com analogia, métricas fake, dashboard Grafana, casos reais, quiz e cola | PromQL |
| 🧩 [**challenges**](challenges/) | **182 desafios PromQL** corrigidos automaticamente: `./check.py 021 'sua query'` compara o **resultado** com o gabarito | PromQL |
| 🧪 [**labs**](labs/) | 9 labs com exercícios "quebre e conserte" e correção automática (tabela abaixo) | todos |
| 🔥 [**gamedays**](gamedays/) | **10 incidentes** em que o monitoramento está sutilmente quebrado: você investiga, conserta e escreve o pós-mortem | Fundamentals, Alerting, PromQL |
| 📝 [**exam**](exam/) | **Simulado**: 60 questões na proporção oficial, 90 min, nota por domínio (página offline + CLI). Banco de ~600 questões | todos |
| 🃏 [**flashcards**](flashcards/) | **~1.100 flashcards** (Anki `.apkg`, CSV e página HTML) para revisão espaçada | todos |
| 🗺️ [**mindmap**](mindmap/) | Mapa mental interativo ([markmap.md](markmap.md)) com links para lição, lab e desafio em cada nó | todos |
| 📊 **Meu progresso** | Seu estudo vira métrica: `./challenges/check.py exporter --docker` + dashboard Grafana http://localhost:3300/d/pca-progress | — |

### Labs

| Lab | Você pratica | Portas |
|---|---|---|
| [promql-operators](labs/promql-operators/) | tipos de dado, matchers, `offset`/`@`, subqueries, `and`/`or`/`unless`, **vector matching** (`on`/`group_left`), agregações | usa 9095 |
| [alertmanager](labs/alertmanager/) | routing tree, `group_by`/`group_wait`, **inhibition**, **silences** (amtool), time intervals, `send_resolved` | 9110-9113 |
| [recording-rules-testing](labs/recording-rules-testing/) | convenção `level:metric:operations`, **`promtool test rules`**, CI | — |
| [service-discovery-relabeling](labs/service-discovery-relabeling/) | static/file_sd/http_sd, **relabel_configs** vs **metric_relabel_configs**, `honor_labels`, limites | 9120-9129 |
| [federation-remote-write](labs/federation-remote-write/) | `/federate`, **remote_write**, agent mode, `external_labels`, remote_read | 9160-9164 |
| [instrumentation](labs/instrumentation/) | counter/gauge/histogram/summary em Go e Python, **naming**, cardinalidade, exemplars, `promtool check metrics` | 9130-9132 |
| [exporters-pushgateway](labs/exporters-pushgateway/) | node_exporter (+ textfile), **blackbox**, **Pushgateway** (quando usar e quando não usar) | 9140-9143 |
| [tsdb-storage](labs/tsdb-storage/) | head/WAL/blocos, retenção, **backfill**, snapshot, `tsdb analyze`, **staleness** | 9150-9151 |
| [slo-end-to-end](labs/slo-end-to-end/) | SLI/SLO/error budget, **multi-window multi-burn-rate**, dashboard | 9170-9171, 3170 |

## ✅ Tudo testado

```bash
./test-all.sh                       # roda o test.sh de todas as partes (~40 min)
./test-all.sh labs/alertmanager     # só uma parte
```

Cada `test.sh` sobe a stack, **aplica as soluções dos exercícios e confere o resultado** (não só "subiu"), e derruba tudo no final.

## 📚 Referências

- Guia rápido da prova (domínios, pesos, top 15 pegadinhas): [promql-functions-lab/PCA.md](promql-functions-lab/PCA.md)
- Mapa mental: [markmap.md](markmap.md)
- Página oficial: https://training.linuxfoundation.org/certification/prometheus-certified-associate/
- Referência externa: https://github.com/onai254/prometheus-certified-associate
