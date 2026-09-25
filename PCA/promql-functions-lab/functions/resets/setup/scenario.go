package main

// Cenário da lição resets().
//
// Valores calculados a partir do relógio de parede: um restart do gerador
// NÃO cria resets "de verdade" (o valor continua de onde estava).
//
// 1) resets_process_cpu_seconds_total{pod} -> imita o process_cpu_seconds_total
//    que toda lib cliente expõe (zera quando o processo reinicia).
//    3 pods usando 0.5 CPU cada (counter sobe 0.5/s):
//    pod="estavel"    nunca reinicia                -> resets(...[10m]) = 0
//    pod="instavel"   reinicia a cada 2 min         -> resets(...[10m]) = 5
//    pod="crashloop"  reinicia a cada 30s           -> resets(...[10m]) = 20
//
// 2) resets_rabbitmq_queue_messages -> GAUGE (fila que sobe e desce, onda de 2 min).
//    Pegadinha: resets() conta TODA queda como reset -> número sem sentido.
//
// 3) resets_kube_pod_container_status_restarts_total{pod="checkout"} -> imita o
//    kube-state-metrics: um counter que CONTA restarts (+1 a cada 2 min).
//    Pegadinha: resets() dele = 0 (o counter nunca cai); o certo é increase().

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "resets",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }

			reg.MustRegister(NewFunc(
				"resets_process_cpu_seconds_total",
				"Segundos de CPU usados pelo processo (0.5/s). Cada restart do pod zera o counter.",
				prometheus.CounterValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := secs(now)
					return []Sample{
						{Labels: []string{"estavel"}, Value: math.Round(0.5*t*100) / 100},
						{Labels: []string{"instavel"}, Value: math.Round(0.5*math.Mod(t, 120)*100) / 100},
						{Labels: []string{"crashloop"}, Value: math.Round(0.5*math.Mod(t, 30)*100) / 100},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"resets_rabbitmq_queue_messages",
				"Mensagens na fila (GAUGE): sobe e desce numa onda de 2 min.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Round(Wave(secs(now), 120, 500, 300, 0) + Noise(20))}}
				},
			))

			reg.MustRegister(NewFunc(
				"resets_kube_pod_container_status_restarts_total",
				"Restarts do container (counter, imita kube-state-metrics): +1 a cada 2 min.",
				prometheus.CounterValue, []string{"pod"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"checkout"}, Value: math.Floor(secs(now) / 120)}}
				},
			))
		},
	})
}
