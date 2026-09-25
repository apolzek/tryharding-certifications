package main

// Cenário da lição stddev_over_time().
//
// stddev_over_time_http_requests_per_second{service} -> gauge que imita uma recording rule
// de tráfego (ex.: job:http_requests:rate1m = sum by (job)(rate(http_requests_total[1m]))).
//   service="checkout" : 100 req/s ± 5 (ruído uniforme -> desvio padrão ≈ 5/√3 ≈ 2.9).
//                       A cada 4 min, durante 20s (≈4 amostras), ANOMALIA: ~160 req/s (ataque de bot).
//   service="search"   : 100 req/s ± 40 (ruído uniforme -> desvio padrão ≈ 40/√3 ≈ 23). Barulhento por natureza.
//   service="legacy"   : 5 req/s CONSTANTE (sistema legado com health-check fixo) -> σ = 0.
//                        Mostra o que dá errado no z-score: 0/0 = NaN.
// checkout e search têm a MESMA média (~100 req/s), mas consistência muito diferente.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "stddev_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"stddev_over_time_http_requests_per_second",
				"Tráfego HTTP em req/s por serviço (gauge; imita uma recording rule de rate).",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					stable := 100 + Noise(5)
					if math.Mod(float64(now.Unix()), 240) < 20 {
						stable = 160 + Noise(3)
					}
					return []Sample{
						{Labels: []string{"checkout"}, Value: stable},
						{Labels: []string{"search"}, Value: 100 + Noise(40)},
						{Labels: []string{"legacy"}, Value: 5},
					}
				},
			))
		},
	})
}
