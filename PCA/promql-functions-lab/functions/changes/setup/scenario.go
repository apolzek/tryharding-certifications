package main

// Cenário da lição changes().
//
// Valores calculados a partir do relógio de parede (determinísticos).
//
// 1) changes_kube_deployment_status_observed_generation{deployment} -> GAUGE
//    (imita o kube-state-metrics: a "generation" sobe 1 a cada rollout/deploy):
//    deployment="api"     deploy a cada 2 min  -> changes(...[10m]) = 5
//    deployment="worker"  deploy a cada 5 min  -> changes(...[10m]) = 2
//    deployment="legacy"  nunca muda (gen 42)  -> changes(...[10m]) = 0
//
// 2) changes_probe_success{target} -> GAUGE 0/1 (imita o blackbox_exporter):
//    target="https://pagamentos.exemplo.com"  sempre 1                     -> changes = 0
//    target="https://frete.exemplo.com"       cai/volta a cada 15s (flap) -> changes(...[5m]) = 20
//
// 3) changes_process_start_time_seconds{pod} -> GAUGE com o timestamp de start
//    do processo (toda client library expõe). pod "api-1" reinicia a cada 3 min:
//    cada restart muda o valor -> changes() = nº de restarts.
//
// 4) changes_http_requests_total -> COUNTER (+3/s). Pegadinha: num counter
//    vivo, changes() só conta amostras (~11 num [1m] com scrape de 5s).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "changes",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }

			reg.MustRegister(NewFunc(
				"changes_kube_deployment_status_observed_generation",
				"Generation observada do Deployment (gauge, imita kube-state-metrics). Sobe 1 a cada rollout.",
				prometheus.GaugeValue, []string{"deployment"},
				func(now time.Time) []Sample {
					t := secs(now)
					return []Sample{
						{Labels: []string{"api"}, Value: 100 + math.Floor(math.Mod(t, 86400)/120)},
						{Labels: []string{"worker"}, Value: 100 + math.Floor(math.Mod(t, 86400)/300)},
						{Labels: []string{"legacy"}, Value: 42},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"changes_probe_success",
				"Resultado do probe HTTP (gauge 0/1, imita blackbox_exporter).",
				prometheus.GaugeValue, []string{"target"},
				func(now time.Time) []Sample {
					t := secs(now)
					return []Sample{
						{Labels: []string{"https://pagamentos.exemplo.com"}, Value: 1},
						{Labels: []string{"https://frete.exemplo.com"}, Value: 1 - Square(t, 30)},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"changes_process_start_time_seconds",
				"Unix timestamp de quando o processo subiu (gauge). api-1 reinicia a cada 3 min.",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"api-1"}, Value: math.Floor(t/180) * 180},
						{Labels: []string{"api-2"}, Value: 1_790_000_000},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"changes_http_requests_total",
				"Requisições HTTP (counter, +3/s). Só para mostrar a pegadinha.",
				prometheus.CounterValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Floor(3 * secs(now))}}
				},
			))
		},
	})
}
