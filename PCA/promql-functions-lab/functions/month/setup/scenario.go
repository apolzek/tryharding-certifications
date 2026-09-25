package main

// Cenário da lição month().
//
// 1) month_simulated_clock_timestamp_seconds -> relógio acelerado:
//    1 mês simulado a cada 10s (dia 10 de cada mês de 2027). Ano inteiro em 2 min.
//    month() dele vira uma escada 1..12.
//
// 2) month_probe_ssl_earliest_cert_expiry{domain} -> expiração de certificados (UTC),
//    sempre relativa ao mês atual para a lição nunca "envelhecer":
//    api.lab.local     dia 14, daqui a 2 meses
//    www.lab.local     dia 3, daqui a 5 meses
//    legacy.lab.local  ÚLTIMO dia deste mês, 23:00 UTC  -> "expira este mês"
//    midnight.lab.local dia 1 do mês que vem, 01:00 UTC -> em UTC é o mês que vem,
//                      mas em Brasília (UTC-3) ainda é o último dia DESTE mês, 22:00!

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "month",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"month_simulated_clock_timestamp_seconds",
				"Relógio acelerado: 1 mês simulado a cada 10s, ano de 2027.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					idx := int(now.Unix()%120) / 10
					return []Sample{{Value: float64(time.Date(2027, time.Month(1+idx), 10, 12, 0, 0, 0, time.UTC).Unix())}}
				},
			))
			reg.MustRegister(NewFunc(
				"month_probe_ssl_earliest_cert_expiry",
				"Timestamp Unix (s, UTC) em que o certificado TLS expira.",
				prometheus.GaugeValue, []string{"domain"},
				func(now time.Time) []Sample {
					u := now.UTC()
					y, m := u.Year(), u.Month()
					ts := func(mm time.Month, d, h int) float64 { return float64(time.Date(y, mm, d, h, 0, 0, 0, time.UTC).Unix()) }
					return []Sample{
						{Labels: []string{"api.lab.local"}, Value: ts(m+2, 14, 12)},
						{Labels: []string{"www.lab.local"}, Value: ts(m+5, 3, 12)},
						{Labels: []string{"legacy.lab.local"}, Value: ts(m+1, 0, 23)}, // dia 0 do próximo = último dia deste
						{Labels: []string{"midnight.lab.local"}, Value: ts(m+1, 1, 1)},
					}
				},
			))
		},
	})
}
