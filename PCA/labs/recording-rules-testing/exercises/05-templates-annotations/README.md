# 05 · Testando annotations e templates (quebre e conserte)

**Objetivo:** o teste exige annotations legíveis:

- `summary: "disco / de host-1 com 5% livre"`
- `description: "restam 5GiB"`

Hoje a regra produz `"disco  de host-1 com 0.05 livre"` e `"restam 0.05 bytes"`. Conserte a **regra** em [`rules.yml`](rules.yml).

## ▶️ Rodar

```bash
cd labs/recording-rules-testing/exercises/05-templates-annotations
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
```

## 💡 Dica

- Qual o nome **real** do label nas `input_series`? Label inexistente vira string vazia, sem erro.
- `$value` é o valor da **expressão do alerta** (aqui, a razão 0.05), não do espaço livre.
- `humanizePercentage` (0.05 → 5%), `humanize1024` (5368709120 → 5Gi).
- Dentro de um template dá para rodar outra query: `{{ with query "..." }}{{ . | first | value }}{{ end }}`.

<details><summary>✅ Solução</summary>

```yaml
        annotations:
          summary: "disco {{ $labels.mountpoint }} de {{ $labels.instance }} com {{ $value | humanizePercentage }} livre"
          description: >-
            restam {{ with printf "node_filesystem_avail_bytes{instance='%s',mountpoint='%s'}" $labels.instance $labels.mountpoint | query }}{{ . | first | value | humanize1024 }}B{{ end }}
```

Testar annotations é o que evita descobrir **na hora do incidente** que a mensagem do pager saiu vazia. Arquivo: [`solutions/05-templates-annotations/rules.yml`](../../solutions/05-templates-annotations/rules.yml).
</details>
