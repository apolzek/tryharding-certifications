package main

// Cenário da lição sum_over_time().
//
// 1) sum_over_time_items_processed_last_scrape{worker} -> gauge que diz
//    "quantos itens o worker processou desde o último scrape (últimos 5s)".
//    batch-a: ~20 por scrape, constante          -> sum_over_time[1m] ≈ 12 × 20 = 240
//    batch-b: 10 por scrape, 30 durante 60s a cada 4 min -> 120, subindo para 360
//
// 2) sum_over_time_container_memory_working_set_bytes -> gauge de NÍVEL (~512 MiB). Somar no tempo
//    não faz sentido: é o exemplo do que NÃO fazer.
//
// 3) sum_over_time_cpu_usage_percent -> senóide 60 ± 30 com período de 4 min.
//    Caso com subquery: sum_over_time((x > bool 80)[8m:5s]) * 5 = segundos acima de 80%.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sum_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"sum_over_time_items_processed_last_scrape",
				"Itens processados desde o último scrape (janela de 5s). Gauge de 'lote'.",
				prometheus.GaugeValue, []string{"worker"},
				func(now time.Time) []Sample {
					b := 10.0
					if now.Unix()%240 < 60 {
						b = 30
					}
					return []Sample{
						{Labels: []string{"batch-a"}, Value: math.Round(20 + Noise(3))},
						{Labels: []string{"batch-b"}, Value: b},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"sum_over_time_container_memory_working_set_bytes",
				"Memória usada (gauge de nível, ~512 MiB). Exemplo do que NÃO somar no tempo.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Round((512 + Noise(8)) * 1024 * 1024)}}
				},
			))

			reg.MustRegister(NewFunc(
				"sum_over_time_cpu_usage_percent",
				"CPU em % (onda de 4 min entre 30 e 90).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Value: math.Round(Wave(t, 240, 60, 30, 0)*10) / 10}}
				},
			))
		},
	})
}
