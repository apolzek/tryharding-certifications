package main

// Cenário da lição info().
//
// 1) target_info{k8s_cluster_name, k8s_namespace_name, version} = 1
//    (ÚNICA exceção à regra de prefixo: info() procura target_info por padrão.)
//    job="lab" e instance="generator:8080" são colocados pelo próprio scrape,
//    exatamente como acontece com um app instrumentado via OpenTelemetry.
//      k8s_cluster_name="prod-sa-east-1", k8s_namespace_name="shop"
//      version alterna "1.4.0" <-> "1.5.0" a cada 3 min (deploy / rollback),
//      para mostrar que um DATA label pode mudar sem quebrar o info().
//
// 2) info_build_info{goversion, revision} = 1 -> um segundo "info metric",
//    que só entra no info() se você pedir via __name__.
//
// 3) info_http_server_requests_total{route} -> counter "de negócio":
//      /api/orders ~20 req/s · /api/users ~5 req/s
//    Ele NÃO tem cluster/namespace/version; info() vai "enriquecer" com esses labels.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "info",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"target_info",
				"Metadados do alvo (estilo OpenTelemetry resource attributes).",
				prometheus.GaugeValue, []string{"k8s_cluster_name", "k8s_namespace_name", "version"},
				func(now time.Time) []Sample {
					v := "1.4.0"
					if Square(float64(now.Unix()), 360) == 1 {
						v = "1.5.0"
					}
					return []Sample{{Labels: []string{"prod-sa-east-1", "shop", v}, Value: 1}}
				},
			))
			reg.MustRegister(NewFunc(
				"info_build_info",
				"Informações de build (outro info metric, fora do padrão target_info).",
				prometheus.GaugeValue, []string{"goversion", "revision"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"go1.25.1", "a1b2c3d"}, Value: 1}}
				},
			))

			// Counter calculado pelo relógio (desde a meia-noite UTC): estável entre restarts.
			reg.MustRegister(NewFunc(
				"info_http_server_requests_total",
				"Total de requisições HTTP atendidas (counter).",
				prometheus.CounterValue, []string{"route"},
				func(now time.Time) []Sample {
					y, m, d := now.UTC().Date()
					midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
					sec := now.Sub(midnight).Seconds()
					return []Sample{
						{Labels: []string{"/api/orders"}, Value: math.Floor(20 * sec), Created: midnight},
						{Labels: []string{"/api/users"}, Value: math.Floor(5 * sec), Created: midnight},
					}
				},
			))
		},
	})
}
