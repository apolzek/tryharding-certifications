# `pi()`: a constante π

> **Em uma frase:** `pi()` não recebe argumentos e devolve o **escalar** `3.141592653589793`.

| | |
|---|---|
| **Assinatura** | `pi() → scalar` |
| **Tipo de métrica** | nenhuma: é uma constante |
| **Unidade do resultado** | número puro (escalar, sem labels) |
| **Dashboard** | http://localhost:3300/d/fn-pi |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a fita métrica do círculo

Tudo que é **redondo** ou **periódico** passa por π:

- perímetro de uma roda: `2 · π · r`;
- área de um tubo ou do círculo varrido por uma pá: `π · r²`;
- uma volta completa em radianos: `2π`;
- uma onda com período P: `sin(2π · t / P)`.

`pi()` é a fita métrica. Detalhe importante: ele é um **escalar**, não um vetor. Operações aritméticas com vetores funcionam normalmente (`2 * pi() * metric`), mas **funções que exigem instant vector** (`sin`, `cos`, `deg`...) precisam de `vector(pi())`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `pi_turbine_rotor_rpm{turbine="T1"}` | gauge | rotação entre **6 e 16 rpm** (período 4 min) |
| `pi_turbine_blade_length_meters{turbine="T1"}` | gauge | pá (raio) de **60 m** |
| `pi_pipe_diameter_meters{pipe}` | gauge | `adutora` = **0.5 m**, `ramal` = **0.1 m** |
| `pi_pipe_flow_velocity_mps{pipe}` | gauge | velocidade da água entre **1 e 3 m/s** (período 3 min) |

**Casos (do básico ao real):** 🟢 **básico:** `pi()` é escalar · 🟡 **nuance:** `vector(pi())`, esquecer `/60` · 🔴 **real:** velocidade da pá, vazão de tubulação

```bash
curl -s localhost:8088/metrics | grep '^pi_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-pi
```

---

## 🔍 Queries passo a passo

### 1 e 2. Velocidade da ponta da pá

```promql
pi_turbine_rotor_rpm
2 * pi() * pi_turbine_blade_length_meters * pi_turbine_rotor_rpm / 60
```

**O que faz:** cada rotação percorre `2πr` metros; multiplicando pelas rotações por segundo (`rpm / 60`) temos m/s.
**Resultado esperado:** 6 rpm → **37.7 m/s**; 16 rpm → **100.5 m/s (≈ 362 km/h!)**. Parece lento no rotor, mas a ponta de uma pá de 60 m é mais rápida que um carro de Fórmula 1.

---

### 3. Vazão de água

```promql
pi() * (pi_pipe_diameter_meters / 2) ^ 2 * pi_pipe_flow_velocity_mps
```

**Resultado esperado:** adutora (área 0.196 m²) entre **0.196 e 0.589 m³/s**; ramal (área 0.00785 m², 25x menor) entre **0.008 e 0.024 m³/s**.

---

### 4. Onda sintética

```promql
sin(vector(2 * pi() * time() / 60))
```

**Resultado esperado:** uma senoide perfeita entre -1 e 1 com **uma volta por minuto**, sem nenhuma métrica. Base para modelos sazonais (ver [`sin()`](../sin/)).

---

### 5. `pi()` é escalar

| Query | Resultado | Observação |
|---|---|---|
| `pi()` | `3.141592653589793` | tipo **scalar** |
| `vector(pi())` | `{} 3.141592653589793` | instant vector sem labels |
| `pi() * pi_turbine_blade_length_meters ^ 2` | `{turbine="T1"} 11309.7` | área varrida (m²) |
| `deg(vector(pi()))` | `180` | |
| `sin(vector(pi()))` | `1.2246467991473515e-16` | não é 0 exato |
| `sin(pi())` | **erro** | `expected type instant vector in call to function "sin", got scalar` |
| `2 * pi() * pi_turbine_blade_length_meters * pi_turbine_rotor_rpm` | `2262 .. 6032` | ❌ **ERRADO**: esqueceu o `/60` (rpm é por **minuto**): a pá "passaria" de 6 000 m/s |

---

## 🏭 Casos reais

> `pi()` aparece em fórmulas geométricas (IoT/indústria) e em modelos sazonais.

**1. Mineração/logística: velocidade linear de correia transportadora.** O exporter do inversor de frequência publica `conveyor_pulley_rpm`; o diâmetro do tambor é fixo (0.8 m). Alerta de **escorregamento** comparando com o sensor de velocidade da correia:

```yaml
groups:
  - name: correias
    rules:
      - record: conveyor:expected_speed:mps
        expr: pi() * 0.8 * conveyor_pulley_rpm / 60
      - alert: ConveyorBeltSlipping
        expr: conveyor_belt_speed_mps / conveyor:expected_speed:mps < 0.9
        for: 2m
        annotations:
          summary: "Correia {{ $labels.conveyor }} escorregando (>10% abaixo do esperado)"
```

**2. Baseline sazonal:** `100 + 50 * sin(vector(2 * pi() * time() / 86400))` como tráfego esperado por hora do dia (ver [`sin()`](../sin/)).

**3. Saneamento: suspeita de vazamento.** Medidores de velocidade em dois pontos da mesma adutora; a vazão de entrada deve igualar a de saída:

```yaml
groups:
  - name: adutora
    rules:
      - record: pipe:flow:m3_per_second
        expr: pi() * (pipe_diameter_meters / 2) ^ 2 * pipe_flow_velocity_mps
      - alert: PipeLeakSuspected
        expr: |
          sum(pipe:flow:m3_per_second{point="entrada"})
            - sum(pipe:flow:m3_per_second{point="saida"}) > 0.05
        for: 15m
        annotations:
          summary: "Diferença de vazão > 50 L/s entre entrada e saída da adutora"
```

## ✅ Quando usar

- Qualquer fórmula com círculo/rotação (perímetro, área, rpm → m/s).
- Converter manualmente ângulos (`x * pi() / 180`), embora [`rad()`](../rad/) seja mais legível.
- Gerar ondas periódicas com `time()`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Converter graus ↔ radianos | [`rad()`](../rad/) / [`deg()`](../deg/) |
| Precisa de um vetor com labels | `vector(pi())` (sem labels) ou multiplique por uma métrica |

## ⚠️ Pegadinhas

1. **É escalar:** `sin(pi())`, `deg(pi())` dão **erro de tipo**. Use `vector(pi())`.
2. **Não aceita argumentos:** `pi(3)` é erro.
3. **Float:** `sin(π)` = 1.22e-16, `cos(π/2)` = 6.1e-17. Nunca compare com `== 0`.
4. **Escalar em range query:** `pi()` num painel vira uma linha reta em 3.14159.

## 🎓 Na prova PCA

- `pi()` retorna um **scalar**; não tem labels.
- Escalar × vetor → vetor (as labels do vetor são mantidas; o nome da métrica é removido pela aritmética).
- Funções que exigem instant vector precisam de `vector(pi())`.

**1.** Qual o tipo de retorno de `pi()`?
- A) instant vector  B) range vector  C) scalar  D) string

<details><summary>Resposta</summary>

**C.** `pi()` é um escalar.
</details>

**2.** Qual expressão é **inválida**?
- A) `2 * pi() * node_load1`
- B) `vector(pi())`
- C) `cos(pi())`
- D) `scalar(vector(pi()))`

<details><summary>Resposta</summary>

**C.** `cos()` exige instant vector; `pi()` é escalar. Correto: `cos(vector(pi()))`.
</details>

**3.** `pi() * my_radius_meters{pipe="a"} ^ 2` retorna uma série com quais labels?
- A) `{__name__="my_radius_meters", pipe="a"}`
- B) `{pipe="a"}`
- C) nenhuma
- D) erro: escalar e vetor não se combinam

<details><summary>Resposta</summary>

**B.** Operações aritméticas entre escalar e vetor mantêm as labels do vetor e removem o nome da métrica.
</details>

## 📝 Cola rápida

- `pi()` → **scalar** 3.141592653589793, sem argumentos.
- Em funções: `sin(vector(pi()))`; em aritmética: `2 * pi() * metric`.
- rpm → m/s: `2 * pi() * r * rpm / 60`; área: `pi() * r ^ 2`.
- Onda: `sin(vector(2 * pi() * time() / P))`.

## 🔗 Relacionadas

[`sin()`](../sin/) · [`cos()`](../cos/) · [`rad()`](../rad/) · [`deg()`](../deg/) · [`vector()`](../vector/) · [`scalar()`](../scalar/) · [`time()`](../time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
