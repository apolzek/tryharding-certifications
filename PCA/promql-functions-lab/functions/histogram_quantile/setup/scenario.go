package main

// Cenário da lição histogram_quantile().
//
// histogram_quantile_http_request_duration_seconds{route, pod}
//   Histograma que expõe AO MESMO TEMPO:
//     - native histogram (schema 3, fator ~1.09)  -> query: histogram_quantile_http_request_duration_seconds
//     - classic histogram (buckets fixos)          -> _bucket{le}, _sum, _count
//
//   route="/api/users"  (pods a e b): ~40 obs/s cada, latência log-normal, mediana 50ms.
//   route="/api/report" (pods a e b): ~5 obs/s cada, mediana 300ms.
//
//   DEGRADAÇÃO PERIÓDICA: a cada 5 min (relógio de parede), durante 90s, o pod-b
//   de /api/users fica lento: mediana 50ms -> 400ms (ex.: GC / vizinho barulhento).
//   p99 do pod-b: 160ms -> ~1.3s. p99 agregado da rota: 160ms -> ~1.1s.
//
// histogram_quantile_coarse_duration_seconds
//   Mesmo tráfego do /api/users pod-a, mas com buckets clássicos GROSSEIROS
//   (0.1, 0.5, 1) para mostrar o erro de interpolação dos clássicos.

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_quantile",
		Setup: func(reg prometheus.Registerer) {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_quantile_http_request_duration_seconds",
				Help:                        "Latência das requisições HTTP (native + classic).",
				Buckets:                     []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"route", "pod"})
			coarse := prometheus.NewHistogram(prometheus.HistogramOpts{
				Name:                        "histogram_quantile_coarse_duration_seconds",
				Help:                        "Mesma latência de /api/users pod-a, mas com buckets clássicos grosseiros.",
				Buckets:                     []float64{0.1, 0.5, 1},
				NativeHistogramBucketFactor: 1.1,
			})
			reg.MustRegister(h, coarse)

			lognormal := func(median, sigma float64) float64 {
				return median * math.Exp(sigma*rand.NormFloat64())
			}
			degraded := func(now time.Time) bool { return now.Unix()%300 < 90 }

			// tick de 100ms: 4 obs (=40/s) em /api/users, 1 obs a cada 2 ticks (=5/s) em /api/report
			tick := 0
			Every(100*time.Millisecond, func() {
				now := time.Now()
				tick++
				for _, pod := range []string{"pod-a", "pod-b"} {
					med := 0.05
					if pod == "pod-b" && degraded(now) {
						med = 0.4
					}
					for i := 0; i < 4; i++ {
						v := lognormal(med, 0.5)
						h.WithLabelValues("/api/users", pod).Observe(v)
						if pod == "pod-a" {
							coarse.Observe(v)
						}
					}
					if tick%2 == 0 {
						h.WithLabelValues("/api/report", pod).Observe(lognormal(0.3, 0.4))
					}
				}
			})
		},
	})
}
