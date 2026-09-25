package main

// Cenário da lição histogram_stdvar() (só native histograms!).
//
// histogram_stdvar_queue_wait_seconds{queue}   (native + classic)
//   Tempo de espera na fila. As duas filas têm MÉDIA de 0.5s.
//   queue="dedicated": máquina dedicada, sigma=0.1 -> desvio ~0.05s, variância ~0.0025 s².
//   queue="shared":    máquina compartilhada com um "vizinho barulhento".
//                      A cada 2 min alterna:
//                        vizinho quieto   (sigma=0.2) -> desvio ~0.10s, variância ~0.010 s²
//                        vizinho barulhento (sigma=0.6) -> desvio ~0.33s, variância ~0.108 s²

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_stdvar",
		Setup: func(reg prometheus.Registerer) {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_stdvar_queue_wait_seconds",
				Help:                        "Tempo de espera na fila (native + classic).",
				Buckets:                     []float64{0.1, 0.25, 0.5, 1, 2.5, 5},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"queue"})
			reg.MustRegister(h)

			ln := func(mean, sigma float64) float64 {
				return mean * math.Exp(-sigma*sigma/2) * math.Exp(sigma*rand.NormFloat64())
			}
			Every(100*time.Millisecond, func() {
				sigma := 0.2
				if Square(float64(time.Now().Unix()), 240) == 0 {
					sigma = 0.6
				}
				for i := 0; i < 4; i++ {
					h.WithLabelValues("dedicated").Observe(ln(0.5, 0.1))
					h.WithLabelValues("shared").Observe(ln(0.5, sigma))
				}
			})
		},
	})
}
