package main

// Cenário da lição sort_by_label() — ordem NATURAL de labels.
//
// 1) sort_by_label_node_load1{zone, node} -> 12 nós (node-1 .. node-12), imita o
//    node_load1 do node_exporter (com os labels zone/node que um relabel colocaria).
//    Nós ímpares em us-east-1a, pares em us-east-1b. load (± 0.05):
//      node-1 1.8 · node-2 0.4 · node-3 2.6 · node-4 1.1 · node-5 0.7 · node-6 2.9
//      node-7 1.5 · node-8 0.2 · node-9 2.2 · node-10 3.1 · node-11 0.9 · node-12 1.3
//    (valores "embaralhados" de propósito: ordem por NOME ≠ ordem por VALOR)
//    Ordem natural: node-2 vem ANTES de node-10 (ordem alfabética pura colocaria
//    node-10, node-11, node-12 antes de node-2).
//
// 2) sort_by_label_app_build_info{pod, version} = 1 -> imita um *_build_info.
//    11 pods: 1 em 1.2.0, 2 em 1.9.2, 5 em 1.10.0, 3 em 1.10.12.
//    count by (version) + sort_by_label -> 1.2.0 < 1.9.2 < 1.10.0 < 1.10.12.

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sort_by_label",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"sort_by_label_node_load1",
				"Load average de 1 minuto por nó (imita node_load1).",
				prometheus.GaugeValue, []string{"zone", "node"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					loads := []float64{1.8, 0.4, 2.6, 1.1, 0.7, 2.9, 1.5, 0.2, 2.2, 3.1, 0.9, 1.3}
					out := make([]Sample, 0, 12)
					for i := 1; i <= 12; i++ {
						zone := "us-east-1a"
						if i%2 == 0 {
							zone = "us-east-1b"
						}
						out = append(out, Sample{
							Labels: []string{zone, fmt.Sprintf("node-%d", i)},
							Value:  Wave(t, 180, loads[i-1], 0.05, float64(i)),
						})
					}
					return out
				},
			))
			reg.MustRegister(NewFunc(
				"sort_by_label_app_build_info",
				"Versão rodando em cada pod (valor sempre 1, estilo *_build_info).",
				prometheus.GaugeValue, []string{"pod", "version"},
				func(now time.Time) []Sample {
					var out []Sample
					add := func(version string, n int) {
						for i := 0; i < n; i++ {
							out = append(out, Sample{Labels: []string{fmt.Sprintf("shop-%s-%d", version, i), version}, Value: 1})
						}
					}
					add("1.2.0", 1)
					add("1.9.2", 2)
					add("1.10.0", 5)
					add("1.10.12", 3)
					return out
				},
			))
		},
	})
}
