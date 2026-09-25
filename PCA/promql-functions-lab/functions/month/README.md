# `month()`: mês do ano (1 = janeiro, em UTC)

> **Em uma frase:** `month(v)` interpreta cada valor de `v` como timestamp Unix e devolve o **mês em UTC**: **1 = janeiro** ... **12 = dezembro**. Sem argumento, usa o instante avaliado ("este mês").

| | |
|---|---|
| **Assinatura** | `month(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (agora) ou gauge cujo **valor é timestamp em segundos** (ex.: `probe_ssl_earliest_cert_expiry`) |
| **Unidade do resultado** | inteiro 1-12 |
| **Dashboard** | http://localhost:3300/d/fn-month |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a folha do calendário na parede do escritório

Aquele calendário de parede com **uma folha por mês**: `month()` diz qual folha está virada. Diferente do JavaScript (`getMonth()` = 0-11), aqui **janeiro é 1**, como gente normal conta.

E, como todas as funções de calendário do Prometheus, a folha é virada em **Londres (UTC)**: no último dia do mês, às 21h de Brasília, o Prometheus já está no mês seguinte.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `month_simulated_clock_timestamp_seconds` | gauge (timestamp) | **relógio acelerado**: 1 mês a cada **10 s** (dia 10 de cada mês de 2027) → ano inteiro em 2 min |
| `month_probe_ssl_earliest_cert_expiry{domain="api.lab.local"}` | gauge (timestamp) | imita o blackbox exporter: expira dia 14, **daqui a 2 meses** |
| `month_probe_ssl_earliest_cert_expiry{domain="www.lab.local"}` | gauge (timestamp) | dia 3, **daqui a 5 meses** |
| `month_probe_ssl_earliest_cert_expiry{domain="legacy.lab.local"}` | gauge (timestamp) | **último dia deste mês**, 23:00 UTC |
| `month_probe_ssl_earliest_cert_expiry{domain="midnight.lab.local"}` | gauge (timestamp) | dia 1 do mês que vem, **01:00 UTC** (em Brasília ainda é o último dia **deste** mês, 22:00) |

As datas são sempre relativas ao mês atual, para a lição nunca "envelhecer".

```bash
curl -s localhost:8088/metrics | grep '^month_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-month
```

---

## 🔍 Queries passo a passo

### 1. Relógio acelerado: mês e trimestre

```promql
month(month_simulated_clock_timestamp_seconds)               # 1 → 12
ceil(month(month_simulated_clock_timestamp_seconds) / 3)     # trimestre 1 → 4
```

**Resultado esperado:** escada de 12 degraus de 10 s (**1 ... 12**) e, por cima, uma escada de 4 degraus de 30 s (trimestres **1 ... 4**).

---

### 2. Mês e ano de expiração de cada certificado

```promql
month(month_probe_ssl_earliest_cert_expiry)              # mês UTC
year(month_probe_ssl_earliest_cert_expiry)               # ano UTC
month(month_probe_ssl_earliest_cert_expiry - 3 * 3600)   # mês em Brasília
```

**Resultado esperado** (em setembro/2026):

| domain | mês UTC | ano | mês Brasília |
|---|---|---|---|
| api.lab.local | **11** | 2026 | 11 |
| www.lab.local | **2** | **2027** | 2 |
| legacy.lab.local | **9** | 2026 | 9 |
| midnight.lab.local | **10** | 2026 | **9** ← pegadinha do fuso |

> Repare no `www`: "mês 2" sozinho é ambíguo. Mês sem ano quase nunca é suficiente.

---

### 3. Alerta: certificados que vencem **este** mês

```promql
(month(month_probe_ssl_earliest_cert_expiry) == scalar(month()))
  and
(year(month_probe_ssl_earliest_cert_expiry) == scalar(year()))
```

**O que faz:** compara o mês **e** o ano de expiração com os de hoje. `scalar()` transforma o `month()` (vetor sem labels) em número, para comparar com o vetor que tem label `domain`.
**Resultado esperado:** só **legacy.lab.local**. O `midnight` fica de fora em UTC, mas para um time no Brasil ele **também** vence "este mês" (dia 30, 22h local).

---

### 4. Hoje

```promql
month()             # 9
ceil(month() / 3)   # 3 (3º trimestre)
```

---

### 5. ❌ O que dá errado: comparar sem `scalar()`

```promql
month(month_probe_ssl_earliest_cert_expiry) == month()
```

**Resultado esperado:** **vazio**, inclusive para o `legacy.lab.local`, que vence este mês. Comparação vetor × vetor casa labels, e `month()` não tem labels. Corrija com `== scalar(month())` ou `== on() group_left month()`.

---

## 🏭 Casos reais

### 1. Relatório "certificados que vencem este mês" (blackbox exporter)

A equipe de segurança faz a renovação em lote no começo de cada mês. O dashboard lista o que vence no mês corrente:

```promql
(month(probe_ssl_earliest_cert_expiry) == scalar(month()))
  and (year(probe_ssl_earliest_cert_expiry) == scalar(year()))
```

Para **alertar**, porém, prefira sempre dias restantes (não depende de calendário):

```yaml
- alert: CertificadoExpiraEm14Dias
  expr: (probe_ssl_earliest_cert_expiry - time()) / 86400 < 14
  for: 1h
```

### 2. Sazonalidade trimestral / fechamento

Limites de capacidade mais altos em dezembro (Natal) para um e-commerce:

```promql
sum(rate(http_requests_total{job="shop"}[5m])) > 5000 and on() (month() != 12)
or
sum(rate(http_requests_total{job="shop"}[5m])) > 12000 and on() (month() == 12)
```

### 3. Recording rule de "trimestre" para dashboards financeiros

```yaml
- record: calendar:quarter
  expr: ceil(month() / 3)
```

---

## ✅ Quando usar

- Agrupar/filtrar por **mês ou trimestre** (sazonalidade, relatórios mensais).
- Extrair o mês de uma data de expiração/criação.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| "Faltam quantos dias?" | `(x - time()) / 86400` com [`time()`](../time/) |
| Comparar com o mês passado | `x offset 30d` (aproximação) |
| Tamanho do mês | [`days_in_month()`](../days_in_month/) |

## ⚠️ Pegadinhas

1. **1 = janeiro** (não 0 como no JavaScript).
2. **Mês sem ano é ambíguo**: fevereiro de 2027 ≠ fevereiro de 2026. Combine com [`year()`](../year/).
3. **UTC:** a virada do mês é às 21h de Brasília no último dia.
4. **`scalar()` / `on()`** para comparar com métricas que têm labels.
5. **Argumento em segundos**; ms → `/ 1000`.

## 🎓 Na prova PCA

- Faixa **1-12 (1 = janeiro)**, UTC, default `vector(time())`.
- Funções de data retornam instant vector **sem o nome da métrica** e com os labels originais.
- Para alertas de expiração, o padrão da prova é `x - time()`, não funções de calendário.

**1.** Quanto vale `month(vector(0))`?
- A) 0
- B) 1
- C) 12
- D) erro

<details><summary>Resposta</summary>

**B.** Timestamp 0 = 1º de janeiro de 1970 UTC, mês 1.
</details>

**2.** Qual expressão retorna o trimestre atual (1 a 4)?
- A) `month() / 4`
- B) `ceil(month() / 3)`
- C) `floor(month() / 3)`
- D) `month() % 3`

<details><summary>Resposta</summary>

**B.** Jan-Mar → 1, Abr-Jun → 2... C daria 0 em jan/fev e 4 em dezembro.
</details>

**3.** Qual é a melhor expressão para **alertar** 14 dias antes de um certificado vencer?
- A) `month(probe_ssl_earliest_cert_expiry) == month()`
- B) `probe_ssl_earliest_cert_expiry - time() < 14 * 86400`
- C) `day_of_month(probe_ssl_earliest_cert_expiry) - day_of_month() < 14`
- D) `probe_ssl_earliest_cert_expiry < 14`

<details><summary>Resposta</summary>

**B.** Diferença de timestamps em segundos. A não casa labels e é grosseira; C quebra na virada do mês; D compara um timestamp com 14.
</details>

## 📝 Cola rápida

- `month()` → **1-12** (1 = jan), UTC.
- Trimestre: `ceil(month() / 3)`.
- "Vence este mês": mês **e** ano iguais, com `scalar()`.
- Alerta de expiração: `x - time()`, não `month()`.

## 🔗 Relacionadas

[`year()`](../year/) · [`day_of_month()`](../day_of_month/) · [`days_in_month()`](../days_in_month/) · [`time()`](../time/) · [`ceil()`](../ceil/) · [`scalar()`](../scalar/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#month
