package main

// Cenário da lição day_of_month().
//
// 1) day_of_month_simulated_clock_timestamp_seconds -> "relógio acelerado":
//    1 dia simulado a cada 10s reais, percorrendo 01/jan/2027 → 28/fev/2027 (59 dias, ~10 min).
//    day_of_month() dele faz um dente-de-serra 1..31, depois 1..28.
//
// 2) day_of_month_invoice_due_timestamp_seconds{customer} -> vencimento de faturas (UTC):
//    acme      dia 5 do mês que vem, 12:00 UTC
//    globex    dia 15 do mês que vem, 12:00 UTC
//    umbrella  dia 1 do mês que vem, 01:00 UTC  (no Brasil ainda é o ÚLTIMO dia do mês, 22:00!)
//
// 3) day_of_month_billing_pending_invoices -> faturas pendentes de envio (gauge ~42).
//    Usada para alertar só nos primeiros dias do mês (fechamento).
//
// 4) day_of_month_js_event_timestamp_milliseconds -> timestamp em MILISSEGUNDOS
//    (vindo de um app JavaScript). Pegadinha: precisa dividir por 1000.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "day_of_month",
		Setup: func(reg prometheus.Registerer) {
			base := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
			reg.MustRegister(NewFunc(
				"day_of_month_simulated_clock_timestamp_seconds",
				"Relógio acelerado (1 dia simulado a cada 10s), jan-fev/2027. Valor = timestamp Unix em segundos.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					k := now.Unix() % 590 // 59 dias * 10s
					return []Sample{{Value: float64(base + k*8640)}}
				},
			))
			reg.MustRegister(NewFunc(
				"day_of_month_invoice_due_timestamp_seconds",
				"Data de vencimento da fatura do cliente (timestamp Unix em segundos, UTC).",
				prometheus.GaugeValue, []string{"customer"},
				func(now time.Time) []Sample {
					u := now.UTC()
					y, m := u.Year(), u.Month()
					ts := func(d, h int) float64 { return float64(time.Date(y, m+1, d, h, 0, 0, 0, time.UTC).Unix()) }
					return []Sample{
						{Labels: []string{"acme"}, Value: ts(5, 12)},
						{Labels: []string{"globex"}, Value: ts(15, 12)},
						{Labels: []string{"umbrella"}, Value: ts(1, 1)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"day_of_month_billing_pending_invoices",
				"Faturas pendentes de envio.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Round(42 + Noise(3))}}
				},
			))
			reg.MustRegister(NewFunc(
				"day_of_month_js_event_timestamp_milliseconds",
				"Último evento do app web, em MILISSEGUNDOS (Date.now() do JavaScript).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: float64((now.Unix() / 60 * 60) * 1000)}}
				},
			))
		},
	})
}
