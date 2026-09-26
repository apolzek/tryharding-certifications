// Serviço fake "checkout" para o lab slo-end-to-end.
//
//	GET /api/checkout       o endpoint "de negócio" (instrumentado)
//	GET /chaos              mostra a injeção de falhas atual (JSON)
//	GET /chaos?error_rate=0.05&latency_ms=300[&latency_ratio=1]   injeta falhas
//	GET /chaos?reset=1      volta ao normal (error_rate=0.0005, latency_ms=0)
//	GET /metrics            métricas Prometheus
//
// Um gerador de carga interno chama /api/checkout LOAD_RPS vezes por segundo (padrão 30),
// então o lab tem tráfego sem precisar de k6/hey.
package main

import (
	"encoding/json"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const baselineErrorRate = 0.0005 // 0,05% de erro "normal": queima metade do budget de um SLO de 99,9%

type chaos struct {
	mu           sync.RWMutex
	ErrorRate    float64 `json:"error_rate"`
	LatencyMs    float64 `json:"latency_ms"`
	LatencyRatio float64 `json:"latency_ratio"`
}

func (c *chaos) get() (float64, float64, float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ErrorRate, c.LatencyMs, c.LatencyRatio
}

var (
	state = &chaos{ErrorRate: baselineErrorRate, LatencyRatio: 1}

	requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Requisições HTTP atendidas, por handler e status code.",
	}, []string{"handler", "code"})

	duration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds",
		Help: "Latência das requisições HTTP.",
		// 0.3 é o limiar do SLO de latência: o bucket PRECISA existir para a razão le="0.3" / count.
		Buckets: []float64{0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 1, 2.5, 5},
	}, []string{"handler"})

	chaosErrorRate = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "chaos_error_rate", Help: "Fração de requisições que o /chaos manda falhar (0..1).",
	})
	chaosLatency = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "chaos_latency_seconds", Help: "Latência extra injetada pelo /chaos.",
	})
)

// Erros determinísticos: um acumulador soma error_rate a cada requisição e, quando passa de 1,
// a requisição falha. Com error_rate=0.05, exatamente 1 a cada 20 falha: SLIs estáveis e previsíveis.
var (
	errMu  sync.Mutex
	errAcc float64
)

func shouldFail(rate float64) bool {
	errMu.Lock()
	defer errMu.Unlock()
	errAcc += rate
	if errAcc >= 1 {
		errAcc -= 1
		return true
	}
	return false
}

func checkout(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	errRate, latMs, latRatio := state.get()

	// latência base: log-normal com mediana 50ms (p99 ≈ 160ms)
	base := 0.05 * math.Exp(0.5*rand.NormFloat64())
	extra := 0.0
	if latMs > 0 && rand.Float64() < latRatio {
		extra = latMs / 1000
	}
	time.Sleep(time.Duration((base + extra) * float64(time.Second)))

	code := 200
	if shouldFail(errRate) {
		code = 500
	}
	w.WriteHeader(code)
	_, _ = w.Write([]byte(http.StatusText(code) + "\n"))

	requests.WithLabelValues("/api/checkout", strconv.Itoa(code)).Inc()
	duration.WithLabelValues("/api/checkout").Observe(time.Since(start).Seconds())
}

func chaosHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state.mu.Lock()
	if q.Get("reset") != "" {
		state.ErrorRate, state.LatencyMs, state.LatencyRatio = baselineErrorRate, 0, 1
	}
	if len(q) > 0 {
		errMu.Lock()
		errAcc = 0.5 // meio caminho: com error_rate alto o primeiro erro vem logo
		errMu.Unlock()
	}
	if v, err := strconv.ParseFloat(q.Get("error_rate"), 64); err == nil {
		state.ErrorRate = math.Max(0, math.Min(v, 1))
	}
	if v, err := strconv.ParseFloat(q.Get("latency_ms"), 64); err == nil {
		state.LatencyMs = math.Max(0, math.Min(v, 10000))
	}
	if v, err := strconv.ParseFloat(q.Get("latency_ratio"), 64); err == nil {
		state.LatencyRatio = math.Max(0, math.Min(v, 1))
	}
	chaosErrorRate.Set(state.ErrorRate)
	chaosLatency.Set(state.LatencyMs / 1000)
	body, _ := json.Marshal(state)
	state.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(append(body, '\n'))
}

func loadGenerator(rps int) {
	client := &http.Client{Timeout: 15 * time.Second}
	tick := time.NewTicker(time.Second / time.Duration(rps))
	for range tick.C {
		go func() {
			resp, err := client.Get("http://127.0.0.1:8080/api/checkout")
			if err == nil {
				resp.Body.Close()
			}
		}()
	}
}

func main() {
	// usado pelo HEALTHCHECK do compose (a imagem distroless não tem wget/curl)
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		resp, err := http.Get("http://127.0.0.1:8080/healthz")
		if err != nil || resp.StatusCode != 200 {
			os.Exit(1)
		}
		os.Exit(0)
	}
	rps := 30
	if v, err := strconv.Atoi(os.Getenv("LOAD_RPS")); err == nil && v > 0 {
		rps = v
	}
	chaosErrorRate.Set(baselineErrorRate)
	// inicializa as séries de erro com 0: sem isso, rate(...{code="500"}) não existe até o 1º erro
	requests.WithLabelValues("/api/checkout", "200")
	requests.WithLabelValues("/api/checkout", "500")

	http.HandleFunc("/api/checkout", checkout)
	http.HandleFunc("/chaos", chaosHandler)
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })

	go loadGenerator(rps)
	log.Printf("checkout fake ouvindo em :8080 (carga interna: %d req/s)", rps)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
