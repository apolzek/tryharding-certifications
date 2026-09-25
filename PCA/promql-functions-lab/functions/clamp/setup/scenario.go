package main

// Cenário da lição clamp().
//
// 1) clamp_sensor_relative_humidity_percent{sensor} -> umidade relativa (%) de sensores
//    ambientais do datacenter (como os expostos via SNMP/Modbus exporters).
//      sala-1 : sensor com defeito de calibração, oscila entre -8% e 108% (período 3 min)
//      sala-2 : sensor bom, oscila entre 50% e 70%
//    clamp(x, 0, 100) força a faixa física 0..100.
//
// 2) clamp_http_requests_total{code} -> imita http_requests_total: 100 req/s, com a
//    fração de erros (code="500") oscilando entre 0% e 0.3% (período 4 min).
//    SLO de 99.9% -> orçamento de erro = 0.1%.
//    "Orçamento restante" = 1 - taxa_de_erro / 0.001  -> vai de 1 até -2.
//    clamp(..., 0, 1) -> 0% .. 100% (um gauge de "combustível" que não fica negativo).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "clamp",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"clamp_sensor_relative_humidity_percent",
				"Umidade relativa medida pelo sensor ambiental, em %.",
				prometheus.GaugeValue, []string{"sensor"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"sala-1"}, Value: Wave(t, 180, 50, 58, 0)},
						{Labels: []string{"sala-2"}, Value: Wave(t, 180, 60, 10, 1)},
					}
				},
			))
			reqs := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "clamp_http_requests_total",
				Help: "Requisições HTTP por código (imita http_requests_total).",
			}, []string{"code"})
			reg.MustRegister(reqs)
			Every(time.Second, func() {
				t := float64(time.Now().Unix())
				f := math.Max(0, Wave(t, 240, 0.0015, 0.0015, 0))
				reqs.WithLabelValues("200").Add(100 * (1 - f))
				reqs.WithLabelValues("500").Add(100 * f)
			})
		},
	})
}
