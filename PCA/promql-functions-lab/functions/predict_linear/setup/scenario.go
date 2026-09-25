package main

// Cenário da lição predict_linear().
//
// Espaço livre em disco, no estilo do node_exporter (node_filesystem_avail_bytes).
// Valores calculados a partir do relógio de parede.
//
// predict_linear_node_filesystem_avail_bytes{mountpoint}
//   "/"         ~40 GiB, estável, mas com ruído de ±300 MiB (ninguém enche)
//   "/var/log"  começa com 20 GiB e perde 0.5 MiB/s (ciclo de 3h, logrotate)
//               -> enche em 11h a 8h: NÃO dispara o alerta de 4h, mas dispara o de 24h
//   "/data"     começa com 12 GiB e perde 1 MiB/s (ciclo de 3h)
//               -> enche em ~3.4h a ~0.4h: DISPARA o alerta clássico "cheio em 4h"
//   "/scratch"  job batch: começa com 20 GiB e perde 40 MiB/s; zera em ~8.5 min,
//               fica cheio e é limpo a cada 10 min. Serve pra VER a previsão
//               "daqui a 5 min" bater no zero ~5 min antes do disco encher.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "predict_linear",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }
			const MiB = 1024 * 1024
			const GiB = 1024 * MiB

			reg.MustRegister(NewFunc(
				"predict_linear_node_filesystem_avail_bytes",
				"Espaço livre no filesystem em bytes (gauge), por ponto de montagem.",
				prometheus.GaugeValue, []string{"mountpoint"},
				func(now time.Time) []Sample {
					t := secs(now)
					root := 40*GiB + Noise(300*MiB)
					varlog := 20*GiB - 0.5*MiB*math.Mod(t, 10800) + Noise(5*MiB)
					data := 12*GiB - 1*MiB*math.Mod(t, 10800) + Noise(5*MiB)
					scratch := math.Max(0, 20*GiB-40*MiB*math.Mod(t, 600))
					return []Sample{
						{Labels: []string{"/"}, Value: math.Round(root)},
						{Labels: []string{"/var/log"}, Value: math.Round(varlog)},
						{Labels: []string{"/data"}, Value: math.Round(data)},
						{Labels: []string{"/scratch"}, Value: math.Round(scratch)},
					}
				},
			))
		},
	})
}
