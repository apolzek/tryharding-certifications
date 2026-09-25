package main

// Cenário da lição year().
//
// 1) year_simulated_clock_timestamp_seconds -> relógio acelerado em volta do Réveillon:
//    1 hora simulada a cada 10s, de 31/dez/2026 12:00 UTC até 01/jan/2027 12:00 UTC (ciclo de 4 min).
//    year() vira 2027 à meia-noite UTC, mas em Brasília (UTC-3) a virada só vem 3 horas
//    simuladas (30s reais) depois.
//
// 2) year_device_manufactured_timestamp_seconds{device} -> data de fabricação (UTC):
//    sensor-a  10/mar/2019
//    router-b  01/jul/2022
//    nobreak-c 01/jan/2026 01:30 UTC  (em Brasília: 31/dez/2025 22:30!)

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "year",
		Setup: func(reg prometheus.Registerer) {
			base := time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC).Unix()
			reg.MustRegister(NewFunc(
				"year_simulated_clock_timestamp_seconds",
				"Relógio acelerado em volta do Réveillon: 1 hora simulada a cada 10s.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					k := now.Unix() % 240
					return []Sample{{Value: float64(base + k*360)}}
				},
			))
			reg.MustRegister(NewFunc(
				"year_device_manufactured_timestamp_seconds",
				"Data de fabricação do dispositivo (timestamp Unix em segundos, UTC).",
				prometheus.GaugeValue, []string{"device"},
				func(now time.Time) []Sample {
					d := func(y int, m time.Month, day, h, min int) float64 {
						return float64(time.Date(y, m, day, h, min, 0, 0, time.UTC).Unix())
					}
					return []Sample{
						{Labels: []string{"sensor-a"}, Value: d(2019, 3, 10, 12, 0)},
						{Labels: []string{"router-b"}, Value: d(2022, 7, 1, 12, 0)},
						{Labels: []string{"nobreak-c"}, Value: d(2026, 1, 1, 1, 30)},
					}
				},
			))
		},
	})
}
