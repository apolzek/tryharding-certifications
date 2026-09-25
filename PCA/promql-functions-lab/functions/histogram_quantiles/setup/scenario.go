package main

// Cenário da lição histogram_quantiles() (experimental, plural!).
//
// histogram_quantiles_api_latency_seconds{service}   (native + classic)
//   service="catalog":  ~30 obs/s, mediana 80ms, estável (cauda curta).
//   service="checkout": ~30 obs/s, mediana 120ms. A cada 4 min, durante 90s,
//                       5% das requisições caem num "caminho lento" (~1.5s,
//                       ex.: timeout no gateway de pagamento). A MEDIANA quase
//                       não muda, mas o p99 explode -> por isso vale olhar
//                       vários quantis de uma vez.

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_quantiles",
		Setup: func(reg prometheus.Registerer) {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_quantiles_api_latency_seconds",
				Help:                        "Latência da API por serviço (native + classic).",
				Buckets:                     []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"service"})
			reg.MustRegister(h)

			ln := func(median, sigma float64) float64 { return median * math.Exp(sigma*rand.NormFloat64()) }
			Every(100*time.Millisecond, func() {
				slowPath := time.Now().Unix()%240 < 90
				for i := 0; i < 3; i++ {
					h.WithLabelValues("catalog").Observe(ln(0.08, 0.3))
					v := ln(0.12, 0.35)
					if slowPath && rand.Float64() < 0.05 {
						v = ln(1.5, 0.2)
					}
					h.WithLabelValues("checkout").Observe(v)
				}
			})
		},
	})
}
