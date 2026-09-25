package main

// Cenário da lição histogram_fraction().
//
// histogram_fraction_http_request_duration_seconds{service}   (native + classic)
//   Buckets clássicos incluem 0.2 e 0.8 de propósito (SLO de 200ms e Apdex T=200ms).
//
//   service="stable":    ~40 obs/s, mediana 60ms  -> ~99% abaixo de 200ms sempre.
//   service="degrading": ~40 obs/s, mediana 80ms  -> ~97% abaixo de 200ms, MAS a
//                        cada 5 min, durante 90s, a mediana vai para 250ms e só
//                        ~30% ficam abaixo de 200ms (o SLO estoura).

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_fraction",
		Setup: func(reg prometheus.Registerer) {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_fraction_http_request_duration_seconds",
				Help:                        "Latência por serviço (native + classic; buckets clássicos com 0.2 e 0.8).",
				Buckets:                     []float64{0.05, 0.1, 0.2, 0.4, 0.8, 1.6, 3.2},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"service"})
			reg.MustRegister(h)

			ln := func(median, sigma float64) float64 { return median * math.Exp(sigma*rand.NormFloat64()) }
			Every(100*time.Millisecond, func() {
				med := 0.08
				if time.Now().Unix()%300 < 90 {
					med = 0.25
				}
				for i := 0; i < 4; i++ {
					h.WithLabelValues("stable").Observe(ln(0.06, 0.5))
					h.WithLabelValues("degrading").Observe(ln(med, 0.5))
				}
			})
		},
	})
}
