package main

// Cenário da lição exp().
//
// 1) exp_prometheus_tsdb_head_series{pod="prometheus-0"} -> imita prometheus_tsdb_head_series
//    durante uma EXPLOSÃO DE CARDINALIDADE: cresce exponencialmente de 10 000 para
//    160 000 séries em 10 min (×16, ou seja, dobra a cada 2.5 min) e volta a 10 000
//    (o label ruim foi removido). Taxa contínua r = ln(16)/600 ≈ 0.00462 /s.
//    Projeção em 2 min: x * exp(r * 120) = x * 1.74  (predict_linear subestima).
//
// 2) exp_fraud_model_score_logit{model="antifraude-v3"} -> saída de um modelo de
//    regressão logística em "log-odds" (logit), oscilando entre -5 e +5 (período 4 min).
//    Probabilidade = 1 / (1 + exp(-logit))  -> de 0.7% a 99.3%.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "exp",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"exp_prometheus_tsdb_head_series",
				"Séries ativas no head block (imita prometheus_tsdb_head_series) durante uma explosão de cardinalidade.",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					k := math.Log(16) / 600
					return []Sample{{Labels: []string{"prometheus-0"}, Value: math.Round(10000 * math.Exp(k*math.Mod(t, 600)))}}
				},
			))
			reg.MustRegister(NewFunc(
				"exp_fraud_model_score_logit",
				"Score do modelo antifraude em log-odds (logit). Probabilidade = 1/(1+exp(-logit)).",
				prometheus.GaugeValue, []string{"model"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Labels: []string{"antifraude-v3"}, Value: Wave(t, 240, 0, 5, 0)}}
				},
			))
		},
	})
}
