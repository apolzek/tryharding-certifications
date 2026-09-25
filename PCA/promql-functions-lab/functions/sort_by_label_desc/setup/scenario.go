package main

// Cenário da lição sort_by_label_desc() — "o mais novo / o maior número primeiro".
//
// 1) sort_by_label_desc_backup_last_size_bytes{date} -> tamanho do backup diário dos
//    últimos 5 dias (imita métricas de Velero/pgBackRest com a data no label).
//    Datas YYYY-MM-DD (UTC) calculadas pelo relógio: "hoje" é sempre o mais novo.
//    Tamanho: 10, 11, 12, 13, 14 GiB (do mais antigo para o de hoje).
//
// 2) sort_by_label_desc_app_build_info{pod, version} = 1 -> 9 pods:
//      2.9.1 -> 1 · 2.10.3 -> 4 · 2.11.0 -> 3 · 2.11.0-rc.1 -> 1 (canary esquecido)
//    Ordem natural decrescente: 2.11.0-rc.1, 2.11.0, 2.10.3, 2.9.1
//    PEGADINHA: ordem natural NÃO é semver (semver diria 2.11.0 > 2.11.0-rc.1).
//
// 3) sort_by_label_desc_kafka_log_size_bytes{topic="orders", partition="0".."11"}
//    (imita kafka_log_log_size). Tamanho = (partição+1) GiB.
//    Desc natural: 11, 10, 9, ... 0 (alfabética daria 9, 8, ..., 2, 11, 10, 1, 0).

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sort_by_label_desc",
		Setup: func(reg prometheus.Registerer) {
			const gib = 1024 * 1024 * 1024
			reg.MustRegister(NewFunc(
				"sort_by_label_desc_backup_last_size_bytes",
				"Tamanho do backup diário (bytes), label date=YYYY-MM-DD.",
				prometheus.GaugeValue, []string{"date"},
				func(now time.Time) []Sample {
					out := make([]Sample, 0, 5)
					for daysAgo := 4; daysAgo >= 0; daysAgo-- {
						d := now.UTC().AddDate(0, 0, -daysAgo).Format("2006-01-02")
						out = append(out, Sample{Labels: []string{d}, Value: float64(14-daysAgo) * gib})
					}
					return out
				},
			))
			reg.MustRegister(NewFunc(
				"sort_by_label_desc_app_build_info",
				"Versão rodando em cada pod (valor sempre 1, estilo *_build_info).",
				prometheus.GaugeValue, []string{"pod", "version"},
				func(now time.Time) []Sample {
					var out []Sample
					add := func(version string, n int) {
						for i := 0; i < n; i++ {
							out = append(out, Sample{Labels: []string{fmt.Sprintf("api-%s-%d", version, i), version}, Value: 1})
						}
					}
					add("2.9.1", 1)
					add("2.10.3", 4)
					add("2.11.0", 3)
					add("2.11.0-rc.1", 1)
					return out
				},
			))
			reg.MustRegister(NewFunc(
				"sort_by_label_desc_kafka_log_size_bytes",
				"Tamanho do log por partição Kafka (imita kafka_log_log_size).",
				prometheus.GaugeValue, []string{"topic", "partition"},
				func(now time.Time) []Sample {
					out := make([]Sample, 0, 12)
					for i := 0; i < 12; i++ {
						out = append(out, Sample{Labels: []string{"orders", fmt.Sprintf("%d", i)}, Value: float64(i+1) * gib})
					}
					return out
				},
			))
		},
	})
}
