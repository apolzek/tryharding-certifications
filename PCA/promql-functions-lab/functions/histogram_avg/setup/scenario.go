package main

// Cenário da lição histogram_avg() (só native histograms!).
//
// histogram_avg_request_duration_seconds{service}   (native + classic)
//   service="web":   ~50 obs/s, média ~50ms. A cada 5 min, durante 60s, fica
//                    lento: média ~200ms (ex.: cache frio).
//   service="batch": ~2 obs/s, média ~1s constante (pouco tráfego, muito lento).
//
//   Serve para mostrar: média por serviço, média GLOBAL correta (ponderada pelo
//   tráfego) x "média das médias" (errada), equivalência com _sum/_count e que
//   a média esconde a cauda.

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_avg",
		Setup: func(reg prometheus.Registerer) {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_avg_request_duration_seconds",
				Help:                        "Duração das requisições por serviço (native + classic).",
				Buckets:                     []float64{0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"service"})
			reg.MustRegister(h)

			// log-normal com MÉDIA (não mediana) = mean
			ln := func(mean, sigma float64) float64 {
				return mean * math.Exp(-sigma*sigma/2) * math.Exp(sigma*rand.NormFloat64())
			}
			tick := 0
			Every(100*time.Millisecond, func() {
				tick++
				webMean := 0.05
				if time.Now().Unix()%300 < 60 {
					webMean = 0.2
				}
				for i := 0; i < 5; i++ {
					h.WithLabelValues("web").Observe(ln(webMean, 0.4))
				}
				if tick%5 == 0 {
					h.WithLabelValues("batch").Observe(ln(1.0, 0.2))
				}
			})
		},
	})
}
