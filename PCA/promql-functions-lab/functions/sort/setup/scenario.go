package main

// Cenário da lição sort() — imita node_exporter (filesystem) e blackbox_exporter (TLS).
//
// 1) sort_node_filesystem_avail_bytes{node, mountpoint} e
//    sort_node_filesystem_size_bytes{node, mountpoint}
//    (imitam node_filesystem_avail_bytes / node_filesystem_size_bytes).
//    % livre = avail/size*100, com ondas de períodos diferentes -> o ranking MUDA:
//      node-a:/       50 ± 35 % (5 min)    size 100 GiB
//      node-a:/data   40 ± 10 % (4 min)    size 500 GiB
//      node-b:/       70 ± 15 % (3 min)    size  50 GiB
//      node-b:/data   20 ± 12 % (5 min)    size   1 TiB
//      node-c:/       60 ± 30 % (3m20s)    size 200 GiB
//
// 2) sort_probe_ssl_earliest_cert_expiry{target} -> timestamp (unix) de expiração do
//    certificado (imita probe_ssl_earliest_cert_expiry do blackbox_exporter).
//    Vence em (dias, a partir da meia-noite UTC de hoje):
//      legacy.example.com 3 · api.example.com 12 · status.example.com 27
//      shop.example.com 45 · admin.example.com 90
//
// 3) sort_cronjob_last_duration_seconds{cronjob} -> duração da última execução:
//      backup 120 · report 45 · cleanup 8 · sync NaN (nunca rodou: 0/0)
//    Mostra onde sort() coloca o NaN (sempre no FIM).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sort",
		Setup: func(reg prometheus.Registerer) {
			const gib = 1024 * 1024 * 1024
			type disk struct {
				node, mnt                    string
				size, base, amp, per, phase float64
			}
			disks := []disk{
				{"node-a", "/", 100 * gib, 50, 35, 300, 0},
				{"node-a", "/data", 500 * gib, 40, 10, 240, 1},
				{"node-b", "/", 50 * gib, 70, 15, 180, 2},
				{"node-b", "/data", 1024 * gib, 20, 12, 300, 3},
				{"node-c", "/", 200 * gib, 60, 30, 200, 4},
			}
			reg.MustRegister(NewFunc(
				"sort_node_filesystem_avail_bytes",
				"Bytes disponíveis por filesystem (imita node_filesystem_avail_bytes).",
				prometheus.GaugeValue, []string{"node", "mountpoint"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					out := make([]Sample, 0, len(disks))
					for _, d := range disks {
						pct := Wave(t, d.per, d.base, d.amp, d.phase)
						out = append(out, Sample{Labels: []string{d.node, d.mnt}, Value: math.Round(d.size * pct / 100)})
					}
					return out
				},
			))
			reg.MustRegister(NewFunc(
				"sort_node_filesystem_size_bytes",
				"Tamanho total por filesystem (imita node_filesystem_size_bytes).",
				prometheus.GaugeValue, []string{"node", "mountpoint"},
				func(now time.Time) []Sample {
					out := make([]Sample, 0, len(disks))
					for _, d := range disks {
						out = append(out, Sample{Labels: []string{d.node, d.mnt}, Value: d.size})
					}
					return out
				},
			))
			reg.MustRegister(NewFunc(
				"sort_probe_ssl_earliest_cert_expiry",
				"Unix timestamp de expiração do certificado TLS (imita blackbox_exporter).",
				prometheus.GaugeValue, []string{"target"},
				func(now time.Time) []Sample {
					y, m, d := now.UTC().Date()
					midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
					exp := func(days int) float64 { return float64(midnight.AddDate(0, 0, days).Unix()) }
					return []Sample{
						{Labels: []string{"https://legacy.example.com"}, Value: exp(3)},
						{Labels: []string{"https://api.example.com"}, Value: exp(12)},
						{Labels: []string{"https://status.example.com"}, Value: exp(27)},
						{Labels: []string{"https://shop.example.com"}, Value: exp(45)},
						{Labels: []string{"https://admin.example.com"}, Value: exp(90)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"sort_cronjob_last_duration_seconds",
				"Duração da última execução de cada CronJob (NaN = nunca rodou).",
				prometheus.GaugeValue, []string{"cronjob"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"backup"}, Value: 120},
						{Labels: []string{"report"}, Value: 45},
						{Labels: []string{"cleanup"}, Value: 8},
						{Labels: []string{"sync"}, Value: math.NaN()},
					}
				},
			))
		},
	})
}
