package main

// Cenário da lição delta().
//
// Valores calculados a partir do relógio de parede.
//
// 1) delta_node_hwmon_temp_celsius{host} -> GAUGE
//    host="zeus"  onda de 5 min entre 40°C e 70°C (esquenta e esfria)
//    host="hera"  estável em ~45°C (±0.3 de ruído)
//
// 2) delta_container_memory_working_set_bytes{pod="api"} -> GAUGE que cresce 1 MiB/s (vazamento)
//    com ruído: ±3 MiB sempre e, em ~10% dos scrapes, um PICO de +150 MiB
//    (alocação temporária). Volta a 200 MiB a cada 10 min (OOM kill).
//    Mostra que delta() usa só o PRIMEIRO e o ÚLTIMO ponto (sofre com ruído),
//    enquanto deriv() usa todos os pontos (regressão linear).
//
// 3) delta_http_requests_total -> COUNTER (+4/s) que reseta a cada 3 min.
//    Pegadinha: delta() em counter fica negativo no reset; increase() não.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "delta",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }
			const MiB = 1024 * 1024

			reg.MustRegister(NewFunc(
				"delta_node_hwmon_temp_celsius",
				"Temperatura da CPU em °C (gauge).",
				prometheus.GaugeValue, []string{"host"},
				func(now time.Time) []Sample {
					t := secs(now)
					return []Sample{
						{Labels: []string{"zeus"}, Value: math.Round((Wave(t, 300, 55, 15, 0)+Noise(0.3))*10) / 10},
						{Labels: []string{"hera"}, Value: math.Round((45+Noise(0.3))*10) / 10},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"delta_container_memory_working_set_bytes",
				"Memória usada pelo pod (gauge). Vaza ~1 MiB/s, com picos de +150 MiB em ~10% dos scrapes; OOM a cada 10 min.",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := secs(now)
					v := 200*MiB + 1*MiB*math.Mod(t, 600) + Noise(3*MiB)
					if Noise(1) > 0.8 { // ~10% das vezes: pico temporário de alocação
						v += 150 * MiB
					}
					return []Sample{{Labels: []string{"api"}, Value: math.Round(v)}}
				},
			))

			reg.MustRegister(NewFunc(
				"delta_http_requests_total",
				"Requisições HTTP (COUNTER, +4/s) que resetam a cada 3 min. Só para a pegadinha.",
				prometheus.CounterValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Floor(4 * math.Mod(secs(now), 180))}}
				},
			))
		},
	})
}
