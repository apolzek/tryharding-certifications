package main

// Cenário da lição floor().
//
// 1) floor_rabbitmq_queue_messages_ready{queue="invoices"} -> imita rabbitmq_queue_messages_ready:
//    mensagens prontas na fila (inteiras). Sobe de 0 a 999 em 3 min e volta a 0
//    (o consumidor esvaziou a fila). O consumidor só processa LOTES CHEIOS de 100:
//    lotes prontos = floor(msgs / 100), sobra = msgs % 100.
//
// 2) floor_process_start_time_seconds{service="api"} -> imita process_start_time_seconds:
//    timestamp Unix de quando o processo subiu. O processo "reinicia" a cada 15 min.
//    uptime = time() - start;  floor(uptime / 60) = minutos COMPLETOS de uptime.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "floor",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"floor_rabbitmq_queue_messages_ready",
				"Mensagens prontas para entrega na fila (imita rabbitmq_queue_messages_ready).",
				prometheus.GaugeValue, []string{"queue"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Labels: []string{"invoices"}, Value: math.Floor(1000 * Saw(t, 180))}}
				},
			))
			reg.MustRegister(NewFunc(
				"floor_process_start_time_seconds",
				"Início do processo em segundos Unix (imita process_start_time_seconds). Reinicia a cada 15 min.",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Labels: []string{"api"}, Value: t - math.Mod(t, 900)}}
				},
			))
		},
	})
}
