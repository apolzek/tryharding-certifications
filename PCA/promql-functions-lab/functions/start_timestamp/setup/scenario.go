package main

// Cenário da lição start_timestamp().
//
// O Prometheus guarda o "start timestamp" (created timestamp) de counters
// quando a flag use-start-timestamps (+ st-storage) está ligada e o alvo
// expõe esse dado (formato protobuf / OpenMetrics "_created").
//
// 1) start_timestamp_http_requests_total{route} -> CounterVec normal do client_golang.
//    O client_golang anota sozinho o created timestamp = quando a série nasceu
//    (= quando o gerador subiu). Serve para "uptime do processo".
//
// 2) start_timestamp_worker_jobs_total{pod} -> counter calculado com Created explícito:
//    worker-a reinicia a cada 4 min (240s) e processa 3 jobs/s
//    worker-b reinicia a cada 7 min (420s) e processa 1.5 jobs/s
//    Em cada restart o counter volta a 0 e o start timestamp muda.
//
//    start_timestamp_process_start_time_seconds{pod} -> o "jeito antigo": gauge com o boot
//    do processo (igual ao process_start_time_seconds), para comparar.
//
// 3) start_timestamp_legacy_jobs_total -> counter SEM created timestamp
//    (exporter antigo). start_timestamp() devolve 0 (1970!).
//
// 4) start_timestamp_queue_depth -> gauge (gauges não têm start timestamp).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "start_timestamp",
		Setup: func(reg prometheus.Registerer) {
			reqs := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "start_timestamp_http_requests_total",
				Help: "Requisições HTTP (CounterVec do client_golang: created timestamp automático).",
			}, []string{"route"})
			reg.MustRegister(reqs)
			Every(time.Second, func() {
				reqs.WithLabelValues("/home").Add(math.Max(0, 8+Noise(2)))
				reqs.WithLabelValues("/login").Add(math.Max(0, 2+Noise(1)))
			})

			reg.MustRegister(NewFunc(
				"start_timestamp_worker_jobs_total",
				"Jobs processados. O pod reinicia periodicamente (counter zera e o created muda).",
				prometheus.CounterValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					var out []Sample
					for _, p := range []struct {
						pod          string
						period, rate float64
					}{{"worker-a", 240, 3}, {"worker-b", 420, 1.5}} {
						born := math.Floor(t/p.period) * p.period
						out = append(out, Sample{
							Labels:  []string{p.pod},
							Value:   p.rate * (t - born),
							Created: time.Unix(int64(born), 0),
						})
					}
					return out
				},
			))

			reg.MustRegister(NewFunc(
				"start_timestamp_process_start_time_seconds",
				"Imita process_start_time_seconds dos workers (gauge com o timestamp de boot).",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"worker-a"}, Value: math.Floor(t/240) * 240},
						{Labels: []string{"worker-b"}, Value: math.Floor(t/420) * 420},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"start_timestamp_legacy_jobs_total",
				"Counter de um exporter antigo, SEM created timestamp.",
				prometheus.CounterValue, []string{"pod"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"legacy-0"}, Value: 2 * math.Mod(float64(now.Unix()), 3600)}}
				},
			))

			reg.MustRegister(NewFunc(
				"start_timestamp_queue_depth",
				"Tamanho da fila (gauge: não tem start timestamp).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Round(Wave(float64(now.Unix()), 180, 50, 20, 0))}}
				},
			))
		},
	})
}
