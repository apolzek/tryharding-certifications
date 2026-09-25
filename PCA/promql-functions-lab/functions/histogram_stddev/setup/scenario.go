package main

// Cenário da lição histogram_stddev() (só native histograms!).
//
// histogram_stddev_request_duration_seconds{service}   (native + classic)
//   Os dois serviços têm a MESMA MÉDIA: 100ms. O que muda é a dispersão.
//   service="stable":  log-normal sigma=0.1  -> desvio padrão ~10ms (sempre).
//   service="erratic": alterna a cada 2 min entre
//                        "calmo"  (sigma=0.3) -> desvio ~31ms
//                        "caos"   (sigma=1.0) -> desvio ~131ms
//                      e a média continua em ~100ms o tempo todo!

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_stddev",
		Setup: func(reg prometheus.Registerer) {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_stddev_request_duration_seconds",
				Help:                        "Latência de dois serviços com a mesma média e dispersões diferentes.",
				Buckets:                     []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"service"})
			reg.MustRegister(h)

			// log-normal com MÉDIA = mean (independente de sigma)
			ln := func(mean, sigma float64) float64 {
				return mean * math.Exp(-sigma*sigma/2) * math.Exp(sigma*rand.NormFloat64())
			}
			Every(100*time.Millisecond, func() {
				sigma := 0.3
				if Square(float64(time.Now().Unix()), 240) == 0 {
					sigma = 1.0
				}
				for i := 0; i < 5; i++ {
					h.WithLabelValues("stable").Observe(ln(0.1, 0.1))
					h.WithLabelValues("erratic").Observe(ln(0.1, sigma))
				}
			})
		},
	})
}
