package main

// Cenário da lição timestamp().
//
// timestamp(v) devolve QUANDO a amostra foi coletada (não o valor dela).
// Para ter amostras "velhas" de propósito, alguns sensores expõem a métrica
// com TIMESTAMP EXPLÍCITO (como faz um exporter que repassa leituras de
// dispositivos IoT, o Pushgateway, ou um federate):
//
// timestamp_sensor_temperature_celsius{sensor}
//   sala-servidores  sem timestamp explícito -> timestamp = hora do scrape (a cada 5s)
//   estufa-lora      rádio LoRa: 1 leitura a cada 60s  (timestamp explícito = múltiplo de 60s)
//   freezer-bateria  bateria fraca: 1 leitura a cada 7 min (420s). Como o lookback do
//                    Prometheus é 5 min, a série SOME das consultas nos 2 últimos minutos do ciclo.
//
// O valor é função do próprio timestamp da leitura, então o mesmo ponto
// raspado várias vezes é idêntico (o Prometheus descarta o duplicado em silêncio).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "timestamp",
		Setup: func(reg prometheus.Registerer) {
			desc := prometheus.NewDesc("timestamp_sensor_temperature_celsius",
				"Temperatura lida pelo sensor (alguns sensores têm timestamp explícito).",
				[]string{"sensor"}, nil)
			reg.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
				now := time.Now()
				t := float64(now.Unix())
				// sensor "normal": timestamp = scrape
				ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, Wave(t, 120, 22, 1, 0), "sala-servidores")
				// sensores com leitura atrasada: timestamp explícito
				for _, s := range []struct {
					name   string
					period float64
					base   float64
				}{{"estufa-lora", 60, 28}, {"freezer-bateria", 420, -18}} {
					ts := math.Floor(t/s.period) * s.period
					v := Wave(ts, 300, s.base, 2, 1)
					m := prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, s.name)
					ch <- prometheus.NewMetricWithTimestamp(time.Unix(int64(ts), 0), m)
				}
			}))
		},
	})
}
