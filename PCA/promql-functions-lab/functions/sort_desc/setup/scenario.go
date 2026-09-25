package main

// Cenário da lição sort_desc() — o clássico "Top N pods por CPU" e ranking de erro.
//
// 1) sort_desc_container_cpu_usage_seconds_total{namespace, pod} -> counter
//    (imita container_cpu_usage_seconds_total do cAdvisor). Velocidade (cores):
//      shop/api-7f9c-a    0.6 ± 0.4 (4 min)     shop/api-7f9c-b  0.7 ± 0.2 (4 min, fase oposta)
//      shop/web-5d8e-x    0.5 ± 0.1             batch/worker-0   0.25 ± 0.15 (3 min)
//      batch/cron-2911    0.15 ± 0.05           shop/redis-0     0.08
//      kube-system/coredns-66bff 0.01
//    api-a e api-b "disputam" o 1º lugar -> o ranking muda com o tempo.
//
// 2) sort_desc_http_requests_total{service, code} -> counter (imita http_requests_total).
//      payments: 88/s code=200 + 12/s code=500  -> erro 12%
//      checkout: 95/s 200 + 5/s 500             -> erro 5%
//      cart:     99/s 200 + 1/s 500             -> erro 1%
//      recommendations: séries existem mas NÃO crescem (0 req/s) -> 0/0 = NaN
//    sort_desc também joga o NaN para o FIM (não para o topo!).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sort_desc",
		Setup: func(reg prometheus.Registerer) {
			// Counters calculados pelo relógio (desde a meia-noite UTC): um restart do
			// gerador não perde incrementos e o rate fica fiel ao roteiro abaixo.
			// CPU(t) = integral de base + amp*sin(2πt/per + phase)  (nunca decresce: base >= amp)
			type p struct {
				ns, pod               string
				base, amp, per, phase float64
			}
			pods := []p{
				{"shop", "api-7f9c-a", 0.6, 0.4, 240, 0},
				{"shop", "api-7f9c-b", 0.7, 0.2, 240, math.Pi},
				{"shop", "web-5d8e-x", 0.5, 0.1, 300, 1},
				{"batch", "worker-0", 0.25, 0.15, 180, 2},
				{"batch", "cron-2911", 0.15, 0.05, 300, 3},
				{"shop", "redis-0", 0.08, 0, 300, 0},
				{"kube-system", "coredns-66bff", 0.01, 0, 300, 0},
			}
			day := func(now time.Time) (time.Time, float64) {
				y, m, d := now.UTC().Date()
				midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
				return midnight, now.Sub(midnight).Seconds()
			}
			reg.MustRegister(NewFunc(
				"sort_desc_container_cpu_usage_seconds_total",
				"Segundos de CPU consumidos por container (counter, imita cAdvisor).",
				prometheus.CounterValue, []string{"namespace", "pod"},
				func(now time.Time) []Sample {
					midnight, sec := day(now)
					out := make([]Sample, 0, len(pods))
					for _, x := range pods {
						k := x.amp * x.per / (2 * math.Pi)
						v := x.base*sec - k*math.Cos(2*math.Pi*sec/x.per+x.phase) + k
						out = append(out, Sample{Labels: []string{x.ns, x.pod}, Value: v, Created: midnight})
					}
					return out
				},
			))

			// recommendations existe, mas nunca recebe tráfego (fica em 0).
			type r struct {
				svc, code string
				speed     float64
			}
			reqs := []r{
				{"payments", "200", 88}, {"payments", "500", 12},
				{"checkout", "200", 95}, {"checkout", "500", 5},
				{"cart", "200", 99}, {"cart", "500", 1},
				{"recommendations", "200", 0}, {"recommendations", "500", 0},
			}
			reg.MustRegister(NewFunc(
				"sort_desc_http_requests_total",
				"Total de requisições HTTP por serviço e status code (counter).",
				prometheus.CounterValue, []string{"service", "code"},
				func(now time.Time) []Sample {
					midnight, sec := day(now)
					out := make([]Sample, 0, len(reqs))
					for _, x := range reqs {
						out = append(out, Sample{Labels: []string{x.svc, x.code}, Value: math.Floor(x.speed * sec), Created: midnight})
					}
					return out
				},
			))
		},
	})
}
