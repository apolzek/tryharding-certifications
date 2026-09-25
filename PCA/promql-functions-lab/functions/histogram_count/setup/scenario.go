package main

// Cenário da lição histogram_count() (só native histograms!).
//
// histogram_count_http_request_duration_seconds{route}   (native + classic)
//   route="/home":  tráfego em ONDA: 20 ± 10 req/s, período de 5 min.
//   route="/login": ~5 req/s, mas a cada 4 min tem uma RAJADA de 60s a ~25 req/s
//                   (ex.: ataque de força bruta). Na rajada, as requisições são
//                   mais lentas (~600ms), então dá pra contar "req lentas/s".
//
// histogram_count(...) conta OBSERVAÇÕES (= requisições), ignorando o valor delas.

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_count",
		Setup: func(reg prometheus.Registerer) {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_count_http_request_duration_seconds",
				Help:                        "Latência das requisições por rota (native + classic).",
				Buckets:                     []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"route"})
			reg.MustRegister(h)

			ln := func(median, sigma float64) float64 { return median * math.Exp(sigma*rand.NormFloat64()) }
			// acumula "fração de observação" para atingir taxas não inteiras por tick
			accHome, accLogin := 0.0, 0.0
			Every(100*time.Millisecond, func() {
				now := float64(time.Now().UnixMilli()) / 1000
				home := Wave(now, 300, 20, 10, 0) // req/s
				login, loginMed := 5.0, 0.08
				if math.Mod(now, 240) < 60 {
					login, loginMed = 25.0, 0.6
				}
				accHome += home / 10
				accLogin += login / 10
				for ; accHome >= 1; accHome-- {
					h.WithLabelValues("/home").Observe(ln(0.06, 0.4))
				}
				for ; accLogin >= 1; accLogin-- {
					h.WithLabelValues("/login").Observe(ln(loginMed, 0.3))
				}
			})
		},
	})
}
