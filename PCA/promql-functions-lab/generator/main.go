package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	reg := prometheus.NewRegistry()
	// Filtro opcional: LAB_SCENARIOS=rate,increase sobe só essas lições.
	only := map[string]bool{}
	for _, s := range strings.Split(os.Getenv("LAB_SCENARIOS"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			only[s] = true
		}
	}
	var loaded []string
	for _, s := range sortedScenarios() {
		if len(only) > 0 && !only[s.Name] {
			continue
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("ERRO no scenario %s: %v", s.Name, r)
				}
			}()
			s.Setup(prometheus.WrapRegistererWith(nil, reg))
			loaded = append(loaded, s.Name)
		}()
	}
	log.Printf("%d scenarios carregados: %s", len(loaded), strings.Join(loaded, ", "))

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{
		EnableOpenMetrics:                   true,
		EnableOpenMetricsTextCreatedSamples: false,
	}))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "PromQL Lab generator — %d scenarios\n\n", len(loaded))
		for _, n := range loaded {
			fmt.Fprintln(w, "-", n)
		}
	})
	log.Println("ouvindo em :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
