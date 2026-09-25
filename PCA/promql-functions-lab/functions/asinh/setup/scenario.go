package main

// Cenário da lição asinh().
//
// asinh_queue_net_growth_per_second{queue}: crescimento LÍQUIDO de filas
// (entradas - saídas por segundo). Pode ser positivo, negativo e zero, e varia
// em ordens de grandeza diferentes:
//   queue="emails":    oscila entre -5 e +5 (período 3 min), cruza o zero.
//   queue="eventos":   oscila entre -50 000 e +50 000 (período 4 min).
//   queue="relatorios": fica quase sempre em 0, com um burst de +2 000 por 30s a cada 5 min.
// Num gráfico linear, "emails" some (parece zero). ln() quebra com negativos
// e zero. asinh() é o "log simétrico" que resolve.
//
// Caso real: asinh_node_timex_offset_seconds{host} imita o
// node_timex_offset_seconds do node_exporter (offset do relógio, com sinal).
// (label "host" e não "instance": "instance" colidiria com o label do alvo
//  e o Prometheus o renomearia para exported_instance.)
//   host="db-1":  NTP saudável, oscila ±0.0003 s (±0.3 ms).
//   host="vm-7":  relógio derivando: rampa de -0.2 s até +2 s a cada 5 min.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "asinh",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"asinh_queue_net_growth_per_second", "Crescimento líquido da fila (msgs/s): entradas - saídas.",
				prometheus.GaugeValue, []string{"queue"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					rep := 0.0
					if math.Mod(t, 300) < 30 {
						rep = 2000
					}
					return []Sample{
						{Labels: []string{"emails"}, Value: Wave(t, 180, 0, 5, 0)},
						{Labels: []string{"eventos"}, Value: Wave(t, 240, 0, 50000, 1)},
						{Labels: []string{"relatorios"}, Value: rep},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"asinh_node_timex_offset_seconds", "Offset do relógio em relação ao NTP (s), com sinal.",
				prometheus.GaugeValue, []string{"host"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{
						{Labels: []string{"db-1"}, Value: Wave(t, 60, 0, 0.0003, 0)},
						{Labels: []string{"vm-7"}, Value: -0.2 + 2.2*Saw(t, 300)},
					}
				},
			))
		},
	})
}
