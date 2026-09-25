package main

// Cenário da lição log10().
//
// 1) log10_http_request_duration_seconds_p99{endpoint} -> imita uma recording rule de
//    p99 por endpoint. As latências cobrem 5 ORDENS DE GRANDEZA:
//      /cache  ~0.0002 s (200 µs)   /health ~0.0015 s (1.5 ms)   /api/users ~0.05 s
//      /search ~0.4 s               /report oscila entre ~1.1 s e ~8.9 s (período 3 min)
//    log10: -3.7, -2.8, -1.3, -0.4, 0.05..0.95.  floor(log10) = "ordem de grandeza".
//
// 2) log10_optical_rx_power_milliwatts{interface} -> imita a potência óptica recebida
//    num transceiver SFP (via SNMP, em mW). dBm = 10 * log10(mW):
//      xe-0/0/1 ~0.5 mW (-3 dBm, saudável)   xe-0/0/2 ~0.05 mW (-13 dBm)
//      xe-0/0/3 degradando de 0.05 mW até 0.002 mW em 5 min (-13 -> -27 dBm, fibra suja)

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "log10",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"log10_http_request_duration_seconds_p99",
				"p99 de latência por endpoint (imita uma recording rule de histogram_quantile).",
				prometheus.GaugeValue, []string{"endpoint"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					j := func(v float64) float64 { return v * (1 + Noise(0.08)) }
					return []Sample{
						{Labels: []string{"/cache"}, Value: j(0.0002)},
						{Labels: []string{"/health"}, Value: j(0.0015)},
						{Labels: []string{"/api/users"}, Value: j(0.05)},
						{Labels: []string{"/search"}, Value: j(0.4)},
						{Labels: []string{"/report"}, Value: math.Pow(10, Wave(t, 180, 0.5, 0.45, 0))},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"log10_optical_rx_power_milliwatts",
				"Potência óptica recebida no transceiver, em mW (imita dados SNMP de DOM/DDM).",
				prometheus.GaugeValue, []string{"interface"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					// -13 dBm -> -27 dBm linearmente em dBm ao longo de 5 min
					dbm := -13 - 14*Saw(t, 300)
					return []Sample{
						{Labels: []string{"xe-0/0/1"}, Value: 0.5 * (1 + Noise(0.02))},
						{Labels: []string{"xe-0/0/2"}, Value: 0.05 * (1 + Noise(0.02))},
						{Labels: []string{"xe-0/0/3"}, Value: math.Pow(10, dbm/10)},
					}
				},
			))
		},
	})
}
