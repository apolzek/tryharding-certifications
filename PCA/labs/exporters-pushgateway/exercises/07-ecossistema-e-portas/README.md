# Exercício 07: ecossistema de exporters e portas padrão

Sem código. Responda e confira (o `test.sh` sonda as portas do lab com o módulo `tcp_connect` do blackbox).

```bash
for hp in prometheus:9090 node-exporter:9100 blackbox:9115 pushgateway:9091; do
  printf '%-20s ' $hp; curl -s "localhost:9142/probe?module=tcp_connect&target=$hp" | grep ^probe_success
done
```

**Perguntas:**

1. Qual a porta padrão de: Prometheus, Alertmanager, node_exporter, Pushgateway, blackbox_exporter?
2. Onde fica o registro oficial de portas padrão?
3. Para monitorar um MySQL, um Redis e um Kafka de terceiros, você instrumenta o código ou usa exporters? Quais?
4. Seu time escreve um serviço Go novo. Exporter ou instrumentação direta?
5. Qual a diferença entre o node_exporter e o blackbox_exporter quanto ao **lugar onde rodam**?

<details><summary>✅ Respostas</summary>

1. Prometheus **9090**, Alertmanager **9093** (cluster 9094), node_exporter **9100**, Pushgateway **9091**, blackbox_exporter **9115**. Outros comuns: mysqld_exporter 9104, postgres_exporter 9187, redis_exporter 9121, snmp_exporter 9116, statsd_exporter 9102 (e 9125 para receber statsd), Grafana 3000.
2. Na wiki do Prometheus: https://github.com/prometheus/prometheus/wiki/Default-port-allocations. Quem escreve um exporter novo reserva a próxima porta livre ali.
3. **Exporters**, porque você não controla o código: `mysqld_exporter`, `redis_exporter`, `kafka_exporter` ou o JMX exporter. Lista oficial: https://prometheus.io/docs/instrumenting/exporters/.
4. **Instrumentação direta** com `client_golang`. Exporter é para quando você **não pode** mudar o código (software de terceiros, kernel, hardware, SaaS). Instrumentação direta dá métricas de negócio, sem processo extra e sem a "tradução" de um exporter.
5. O node_exporter roda **em cada máquina** (um por nó; no Kubernetes, DaemonSet) e lê o `/proc` e o `/sys` locais. O blackbox roda em **poucos pontos de observação** e sonda alvos **remotos** de fora para dentro, por isso usa o padrão multi-target (`/probe?target=`).
</details>
