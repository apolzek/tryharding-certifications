package main

// Cenário da lição clamp_min().
//
// 1) clamp_min_kube_resourcequota{namespace="loja", resource="requests.cpu", type}
//    -> imita kube_resourcequota (kube-state-metrics):
//       type="hard" = 20 cores (limite da quota; foi REDUZIDO recentemente)
//       type="used" = oscila entre 12 e 26 cores (período 4 min)
//    "Restante" = hard - used -> fica NEGATIVO quando used > hard (até -6).
//    clamp_min(..., 0) -> "sobrou 0 cores" em vez de "sobrou -6 cores".
//
// 2) clamp_min_rabbitmq_queue_messages{queue="emails"} -> ~600 mensagens na fila.
//    clamp_min_kube_deployment_status_replicas_available{deployment="email-worker"}
//    -> 3 réplicas, mas a cada 3 min o deployment fica 30 s com 0 réplicas (rollout ruim).
//    msgs / réplicas -> +Inf quando réplicas = 0.  msgs / clamp_min(réplicas, 1) -> finito.
//
// 3) clamp_min_node_filesystem_avail_bytes{mountpoint="/var/lib/postgresql"}
//    -> imita node_filesystem_avail_bytes: o espaço livre cai de 40 GiB até 4 GiB em 15 min (2.4 GiB/min).
//    predict_linear(...[2m], 600) fica NEGATIVO ("-20 GiB livres daqui a 10 min").
//    clamp_min(predict_linear(...), 0) -> "0 bytes livres" (o disco enche).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "clamp_min",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"clamp_min_kube_resourcequota",
				"Quota de recursos do namespace (imita kube_resourcequota).",
				prometheus.GaugeValue, []string{"namespace", "resource", "type"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"loja", "requests.cpu", "hard"}, Value: 20},
						{Labels: []string{"loja", "requests.cpu", "used"}, Value: math.Round(Wave(t, 240, 19, 7, 0)*10) / 10},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"clamp_min_rabbitmq_queue_messages",
				"Mensagens na fila (imita rabbitmq_queue_messages).",
				prometheus.GaugeValue, []string{"queue"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"emails"}, Value: math.Round(600 + Noise(30))}}
				},
			))
			reg.MustRegister(NewFunc(
				"clamp_min_kube_deployment_status_replicas_available",
				"Réplicas disponíveis (imita kube_deployment_status_replicas_available). Cai a 0 por 30 s a cada 3 min.",
				prometheus.GaugeValue, []string{"deployment"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					v := 3.0
					if math.Mod(t, 180) < 30 {
						v = 0
					}
					return []Sample{{Labels: []string{"email-worker"}, Value: v}}
				},
			))
			reg.MustRegister(NewFunc(
				"clamp_min_node_filesystem_avail_bytes",
				"Espaço livre no filesystem (imita node_filesystem_avail_bytes). Cai de 40 GiB a 4 GiB em 15 min.",
				prometheus.GaugeValue, []string{"mountpoint"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					gib := 40 - 36*Saw(t, 900)
					return []Sample{{Labels: []string{"/var/lib/postgresql"}, Value: gib * 1024 * 1024 * 1024}}
				},
			))
		},
	})
}
