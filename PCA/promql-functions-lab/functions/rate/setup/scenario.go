package main

// Cenário da lição rate().
//
// 1) rate_http_requests_total{route}  -> counter "vivo" que cresce a cada 1s
//    /home        ~10 req/s constante
//    /checkout    ~2 req/s constante
//    /api/search  ~5 req/s, mas com um PICO de ~40 req/s durante 60s a cada 5 min
//
// 2) rate_worker_jobs_processed_total{pod} -> counter que "reinicia" (volta a 0)
//    a cada 3 min, simulando o restart de um pod. Velocidade real: 2 jobs/s.
//    Serve pra mostrar que rate() compensa resets automaticamente.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "rate",
		Setup: func(reg prometheus.Registerer) {
			reqs := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "rate_http_requests_total",
				Help: "Total de requisições HTTP recebidas (counter).",
			}, []string{"route"})
			reg.MustRegister(reqs)

			speed := func(route string, now time.Time) float64 {
				switch route {
				case "/home":
					return 10
				case "/checkout":
					return 2
				default: // /api/search: pico de 60s a cada 300s
					if math.Mod(float64(now.Unix()), 300) < 60 {
						return 40
					}
					return 5
				}
			}
			Every(time.Second, func() {
				now := time.Now()
				for _, r := range []string{"/home", "/checkout", "/api/search"} {
					reqs.WithLabelValues(r).Add(math.Max(0, speed(r, now)+Noise(1)))
				}
			})

			reg.MustRegister(NewFunc(
				"rate_worker_jobs_processed_total",
				"Jobs processados pelo worker. O pod reinicia a cada 3 min (counter volta a zero).",
				prometheus.CounterValue, []string{"pod"},
				func(now time.Time) []Sample {
					sinceRestart := math.Mod(float64(now.Unix()), 180)
					return []Sample{{Labels: []string{"worker-a"}, Value: 2 * sinceRestart}}
				},
			))
		},
	})
}
