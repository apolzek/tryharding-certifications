# `year()`: o ano (em UTC) de um timestamp

> **Em uma frase:** `year(v)` devolve o **ano em UTC** de cada timestamp de `v` (segundos Unix). Sem argumento, usa o instante avaliado ("este ano").

| | |
|---|---|
| **Assinatura** | `year(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (agora) ou gauge cujo **valor é timestamp em segundos** |
| **Unidade do resultado** | ano (ex.: `2026`) |
| **Dashboard** | http://localhost:3300/d/fn-year |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o ano cunhado na moeda... em Londres

Toda moeda tem o **ano de cunhagem** gravado. `year()` lê esse número a partir de um timestamp.

A casa da moeda do Prometheus fica em **Londres (UTC)**: o **Réveillon do Prometheus** acontece às **21h de 31 de dezembro** em Brasília. Uma venda feita às 22h30 de 31/12/2025 em São Paulo é "de 2026" para o `year()`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `year_simulated_clock_timestamp_seconds` | gauge (timestamp) | **Réveillon acelerado**: 1 hora simulada a cada **10 s**, de 31/dez/2026 12:00 UTC a 01/jan/2027 12:00 UTC (ciclo de 4 min) |
| `year_device_manufactured_timestamp_seconds{device="sensor-a"}` | gauge (timestamp) | fabricado em 10/mar/**2019** |
| `year_device_manufactured_timestamp_seconds{device="router-b"}` | gauge (timestamp) | 01/jul/**2022** |
| `year_device_manufactured_timestamp_seconds{device="nobreak-c"}` | gauge (timestamp) | 01/jan/2026 **01:30 UTC** = 31/dez/**2025** 22:30 em Brasília |

```bash
curl -s localhost:8088/metrics | grep '^year_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-year
```

Em 4 min você vê um Réveillon completo.

---

## 🔍 Queries passo a passo

### 1. Réveillon acelerado: UTC vs Brasília

```promql
year(year_simulated_clock_timestamp_seconds)               # UTC
year(year_simulated_clock_timestamp_seconds - 3 * 3600)    # Brasília
```

**Resultado esperado:** dois degraus **2026 → 2027**. O de UTC sobe no meio do ciclo (meia-noite UTC, 120 s depois do início); o de Brasília sobe **30 s depois** (3 "horas" aceleradas). Nesses 30 s, UTC já está em 2027 e Brasília ainda em 2026.

---

### 2. Hoje

```promql
year()     # 2026
```

---

### 3. Ano de fabricação: UTC vs Brasília

```promql
year(year_device_manufactured_timestamp_seconds)              # UTC
year(year_device_manufactured_timestamp_seconds - 3 * 3600)   # Brasília
```

**Resultado esperado:**

| device | UTC | Brasília |
|---|---|---|
| sensor-a | 2019 | 2019 |
| router-b | 2022 | 2022 |
| nobreak-c | **2026** | **2025** ← a nota fiscal diz 2025 |

---

### 4. Idade: "anos de calendário" vs anos exatos

```promql
scalar(year()) - year(year_device_manufactured_timestamp_seconds)       # viradas de ano
(time() - year_device_manufactured_timestamp_seconds) / (365.25 * 86400)  # idade real
```

**Resultado esperado** (set/2026):

| device | anos de calendário | anos exatos |
|---|---|---|
| sensor-a | **7** | ≈ **7,55** |
| router-b | **4** | ≈ **4,24** |
| nobreak-c | **0** | ≈ **0,73** |

A primeira conta **quantos Réveillons** passaram; a segunda é a idade de verdade. Para "equipamento com mais de 5 anos → trocar", use a segunda.

---

## 🏭 Casos reais

### 1. Inventário: hardware fora da garantia (5 anos)

Um exporter de inventário (ou o `node_dmi_info` + uma métrica de data de compra) expõe a data de aquisição:

```yaml
- alert: HardwareForaDaGarantia
  expr: (time() - asset_purchase_timestamp_seconds) / (365.25 * 86400) > 5
  labels: {severity: info}
```

E um painel "quantos servidores foram comprados há 5+ viradas de ano":

```promql
count(year(asset_purchase_timestamp_seconds) <= scalar(year()) - 5)
```

(`year()` devolve o ano como **valor**, não como label: para um gráfico "servidores por ano de compra" com `count by (ano)`, o ano precisa ser um **label** publicado pelo exporter.)

### 2. Certificados que vencem no ano que vem (planejamento de orçamento)

```promql
count(year(probe_ssl_earliest_cert_expiry) == scalar(year()) + 1)
```

### 3. Recording rule de ano fiscal

Empresas com ano fiscal começando em abril:

```yaml
- record: calendar:fiscal_year
  expr: year(vector(time() - 90 * 86400))   # aproximação: desloca ~3 meses
```

---

## ✅ Quando usar

- Extrair o ano de datas (fabricação, compra, expiração).
- Comparar "mesmo ano" junto com [`month()`](../month/).
- Idade em "anos de calendário".

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Idade exata em anos | `(time() - x) / (365.25 * 86400)` |
| Comparar com o ano passado | `x offset 1y` (ou `365d`) |
| Dia/mês | [`day_of_year()`](../day_of_year/), [`month()`](../month/) |

## ⚠️ Pegadinhas

1. **UTC:** o ano vira às 21h de 31/12 em Brasília. Relatórios fiscais brasileiros precisam de `year(x - 3*3600)`.
2. **`year() - year(x)` não é idade**: de 31/12 para 01/01 já dá 1 "ano".
3. **ms em vez de s** → ano ~58 000. Divida por 1000.
4. **Vetor sem labels vs com labels** → `scalar(year())` para subtrair.
5. `year()` devolve o ano como **valor**, não como label; agrupar por ano exige que o ano seja label na origem.

## 🎓 Na prova PCA

- `year()` UTC, default `vector(time())`, recebe/retorna instant vector.
- Diferença entre **valor** e **label**: funções operam em valores; agregação `by` usa labels.
- Aritmética escalar × vetor e a necessidade de `scalar()`.

**1.** Quanto vale `year(vector(1767225600))` (01/01/2026 00:00 UTC)?
- A) 2025
- B) 2026
- C) depende do fuso do servidor
- D) erro

<details><summary>Resposta</summary>

**B.** O PromQL sempre usa UTC; o fuso do servidor não influencia.
</details>

**2.** Qual expressão dá a idade de um dispositivo em anos, com fração?
- A) `year() - year(manufactured_ts)`
- B) `(time() - manufactured_ts) / (365.25 * 86400)`
- C) `year(time() - manufactured_ts)`
- D) `rate(manufactured_ts[1y])`

<details><summary>Resposta</summary>

**B.** A conta viradas de ano (e ainda não casa labels sem `scalar()`); C é erro de tipo (escalar) e, se fosse vetor, daria algo como "1977"; D não faz sentido para um gauge constante.
</details>

**3.** O que retorna `year()` às 22h de 31/12/2025 em São Paulo?
- A) 2025
- B) 2026
- C) 2024
- D) vazio

<details><summary>Resposta</summary>

**B.** 22h BRT = 01h UTC de 01/01/2026.
</details>

## 📝 Cola rápida

- `year()` → ano em **UTC**. Réveillon do Prometheus = 21h BRT.
- Brasília: `year(x - 3*3600)` / `year(vector(time() - 3*3600))`.
- Idade real: `(time() - x) / (365.25*86400)`, não `year() - year(x)`.
- Mês sem ano é ambíguo → combine `month()` + `year()`.

## 🔗 Relacionadas

[`month()`](../month/) · [`day_of_year()`](../day_of_year/) · [`time()`](../time/) · [`scalar()`](../scalar/) · [`label_replace()`](../label_replace/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#year
