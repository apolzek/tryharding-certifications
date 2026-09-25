package main

// Cenário da lição histogram_sum() (só native histograms!).
//
// 1) histogram_sum_http_response_size_bytes{route}   (native + classic)
//    route="/api/items": ~50 resp/s de ~2 KB   -> ~100 KB/s
//    route="/download":  ~1 resp/s de ~5 MB    -> ~5 MB/s
//    histogram_sum(rate(...)) = BYTES POR SEGUNDO enviados (throughput).
//
// 2) histogram_sum_job_duration_seconds{queue}       (native + classic)
//    queue="emails":  10 jobs/s de ~0.2s  -> 2 "segundos de trabalho por segundo"
//    queue="reports": 0.5 job/s de ~8s    -> 4 workers ocupados em média.
//                     A cada 5 min, por 90s, os relatórios ficam 2x mais lentos
//                     (~16s) -> ~8 workers ocupados, com o MESMO número de jobs.
//    histogram_sum(rate(...)) de uma duração = CONCORRÊNCIA média (Lei de Little).

import (
	"math"
	"math/rand/v2"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "histogram_sum",
		Setup: func(reg prometheus.Registerer) {
			size := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_sum_http_response_size_bytes",
				Help:                        "Tamanho das respostas HTTP em bytes (native + classic).",
				Buckets:                     []float64{512, 1024, 4096, 16384, 65536, 1 << 20, 4 << 20, 16 << 20},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"route"})
			jobs := prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:                        "histogram_sum_job_duration_seconds",
				Help:                        "Duração de jobs por fila (native + classic).",
				Buckets:                     []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 40},
				NativeHistogramBucketFactor: 1.1,
			}, []string{"queue"})
			reg.MustRegister(size, jobs)

			// log-normal com MÉDIA = mean
			ln := func(mean, sigma float64) float64 {
				return mean * math.Exp(-sigma*sigma/2) * math.Exp(sigma*rand.NormFloat64())
			}
			tick := 0
			Every(100*time.Millisecond, func() {
				tick++
				for i := 0; i < 5; i++ {
					size.WithLabelValues("/api/items").Observe(ln(2000, 0.3))
				}
				if tick%10 == 0 {
					size.WithLabelValues("/download").Observe(ln(5e6, 0.2))
				}
				jobs.WithLabelValues("emails").Observe(ln(0.2, 0.3))
				if tick%20 == 0 {
					d := 8.0
					if time.Now().Unix()%300 < 90 {
						d = 16
					}
					jobs.WithLabelValues("reports").Observe(ln(d, 0.15))
				}
			})
		},
	})
}
