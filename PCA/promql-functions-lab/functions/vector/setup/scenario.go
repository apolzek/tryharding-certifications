package main

// Cenário da lição vector() — o clássico "or vector(0)" em SLO de taxa de erro.
//
// vector_http_requests_total{route, code} -> counter (imita http_requests_total).
//   code="200" existe SEMPRE:        /checkout ≈ 15/s · /search ≈ 25/s
//   code="500" só existe 2 min a cada 5 min (minutos 0-2 de cada ciclo de 5 min
//   no relógio): /checkout ≈ 3/s · /search ≈ 1/s.
//   Fora disso as séries de erro simplesmente NÃO EXISTEM, como acontece com
//   counters que só nascem no primeiro erro, ou que somem quando o pod reinicia.
//   (valores calculados pelo relógio: restart do gerador não bagunça a lição)
//
// Mostra: o "No data" do painel de erro, o remédio `or vector(0)`, e a pegadinha
// de `sum by (route) (...) or vector(0)` (vector(0) não tem labels!).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "vector",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"vector_http_requests_total",
				"Total de requisições HTTP por rota e status code (counter). Erros 500 só existem 2 min a cada 5 min.",
				prometheus.CounterValue, []string{"route", "code"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					// counters "OK" contam desde a meia-noite UTC (estáveis entre restarts)
					y, m, d := now.UTC().Date()
					midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
					sinceMidnight := now.Sub(midnight).Seconds()
					out := []Sample{
						{Labels: []string{"/checkout", "200"}, Value: math.Floor(15 * sinceMidnight), Created: midnight},
						{Labels: []string{"/search", "200"}, Value: math.Floor(25 * sinceMidnight), Created: midnight},
					}
					in := math.Mod(t, 300)
					if in < 120 {
						// início exato do ciclo (determinístico: o start timestamp não pode "tremer" entre scrapes)
						born := time.Unix(int64(t)-int64(in), 0)
						out = append(out,
							Sample{Labels: []string{"/checkout", "500"}, Value: math.Floor(3 * in), Created: born},
							Sample{Labels: []string{"/search", "500"}, Value: math.Floor(1 * in), Created: born},
						)
					}
					return out
				},
			))
		},
	})
}
