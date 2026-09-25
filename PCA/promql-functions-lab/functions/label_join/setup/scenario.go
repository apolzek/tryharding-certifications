package main

// Cenário da lição label_join() — imita um http_requests_total real + um "CMDB".
//
// 1) label_join_http_requests_total{service, environment, region} -> counter (cresce a cada 1s).
//    Velocidade (rate):
//      checkout/prod    ≈ 150 req/s  (region="sa-east-1")
//      checkout/staging ≈ 10  req/s  (region vazio -> label ausente!)
//      payments/prod    ≈ 80  req/s  (region="sa-east-1")
//      payments/staging ≈ 5   req/s  (region vazio -> label ausente!)
//    (Não usamos "env": o scrape deste lab já coloca env="lab" em tudo.)
//
// 2) label_join_capacity_rps{key} -> capacidade planejada, indexada por UMA chave
//    composta "service/environment" (como vem de um CMDB / planilha de capacity planning):
//      checkout/prod 200 · checkout/staging 50 · payments/prod 100 · payments/staging 20
//    Com label_join fazemos o join -> utilização 75% · 20% · 80% · 25%.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "label_join",
		Setup: func(reg prometheus.Registerer) {
			// Counter calculado pelo relógio (conta desde a meia-noite UTC): um restart
			// do gerador não "perde" requisições e o rate fica estável.
			type s struct {
				svc, envName, region string
				speed                float64
			}
			series := []s{
				{"checkout", "prod", "sa-east-1", 150},
				{"checkout", "staging", "", 10},
				{"payments", "prod", "sa-east-1", 80},
				{"payments", "staging", "", 5},
			}
			reg.MustRegister(NewFunc(
				"label_join_http_requests_total",
				"Total de requisições HTTP por serviço/ambiente/região (counter).",
				prometheus.CounterValue, []string{"service", "environment", "region"},
				func(now time.Time) []Sample {
					y, m, d := now.UTC().Date()
					midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
					sec := now.Sub(midnight).Seconds()
					out := make([]Sample, 0, len(series))
					for _, x := range series {
						// pequena ondulação (±5%, período 4 min) sem nunca decrescer
						v := x.speed*sec - 0.05*x.speed*240/(2*math.Pi)*math.Cos(2*math.Pi*sec/240)
						out = append(out, Sample{Labels: []string{x.svc, x.envName, x.region}, Value: math.Floor(v + x.speed*2), Created: midnight})
					}
					return out
				},
			))
			reg.MustRegister(NewFunc(
				"label_join_capacity_rps",
				"Capacidade planejada (req/s) indexada pela chave composta service/environment.",
				prometheus.GaugeValue, []string{"key"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"checkout/prod"}, Value: 200},
						{Labels: []string{"checkout/staging"}, Value: 50},
						{Labels: []string{"payments/prod"}, Value: 100},
						{Labels: []string{"payments/staging"}, Value: 20},
					}
				},
			))
		},
	})
}
