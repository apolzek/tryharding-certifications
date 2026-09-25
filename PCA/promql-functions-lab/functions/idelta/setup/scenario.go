package main

// Cenário da lição idelta().
//
// Valores calculados a partir do relógio de parede.
//
// 1) idelta_rabbitmq_queue_messages{queue="emails"} -> GAUGE (imita o exporter
//    do RabbitMQ) em "dente de serra invertido":
//    a cada 30s chega um LOTE de 500 mensagens de uma vez; os consumidores
//    drenam 20 msg/s (= 100 por scrape de 5s) até zerar.
//    idelta() mostra o que mudou ENTRE OS DOIS ÚLTIMOS SCRAPES:
//    ~ -100 durante a drenagem, um salto de ~ +400/+500 quando o lote chega.
//
// 2) idelta_pg_stat_activity_count{datname="shop"} -> GAUGE (imita o
//    postgres_exporter: conexões abertas no banco) que fica parado e dá
//    degraus: +20 conexões a cada 45s, voltando a 10 a cada 3 min.
//
// 3) idelta_http_requests_total -> COUNTER (+4/s) que reseta a cada 2 min.
//    Pegadinha: idelta() não entende reset (fica negativo); irate() entende.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "idelta",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }

			reg.MustRegister(NewFunc(
				"idelta_rabbitmq_queue_messages",
				"Mensagens na fila (gauge). Lote de 500 a cada 30s, drenado a 20 msg/s.",
				prometheus.GaugeValue, []string{"queue"},
				func(now time.Time) []Sample {
					v := math.Max(0, 500-20*math.Mod(secs(now), 30))
					return []Sample{{Labels: []string{"emails"}, Value: math.Round(v)}}
				},
			))

			reg.MustRegister(NewFunc(
				"idelta_pg_stat_activity_count",
				"Conexões abertas no banco (gauge, imita postgres_exporter). +20 a cada 45s, volta a 10 a cada 3 min.",
				prometheus.GaugeValue, []string{"datname"},
				func(now time.Time) []Sample {
					steps := math.Floor(math.Mod(secs(now), 180) / 45)
					return []Sample{{Labels: []string{"shop"}, Value: 10 + 20*steps}}
				},
			))

			reg.MustRegister(NewFunc(
				"idelta_http_requests_total",
				"Requisições HTTP (COUNTER, +4/s) que resetam a cada 2 min. Só para a pegadinha.",
				prometheus.CounterValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Floor(4 * math.Mod(secs(now), 120))}}
				},
			))
		},
	})
}
