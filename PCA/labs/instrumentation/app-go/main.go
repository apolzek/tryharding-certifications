// app-go: uma loja fake instrumentada com client_golang.
//
// Expõe TRÊS endpoints de métricas:
//
//	/metrics       -> tudo (métricas BOAS + métricas RUINS), é o que o Prometheus raspa
//	/metrics/good  -> só as boas (promtool check metrics passa)
//	/metrics/bad   -> só as ruins (promtool check metrics reclama)
//
// As métricas ruins estão marcadas com "RUIM" e são o alvo dos exercícios.
package main

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	mrand "math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	good = prometheus.NewRegistry()
	bad  = prometheus.NewRegistry()

	// ─────────────── BOAS ───────────────

	// COUNTER: só sobe. Sufixo _total. Labels de cardinalidade baixa e limitada.
	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total de requisições HTTP atendidas.",
	}, []string{"method", "route", "code"})

	// HISTOGRAM: latência em SEGUNDOS (unidade base). Buckets clássicos
	// (DefBuckets) + native histogram (NativeHistogramBucketFactor).
	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:                        "http_request_duration_seconds",
		Help:                        "Latência das requisições HTTP.",
		Buckets:                     prometheus.DefBuckets,
		NativeHistogramBucketFactor: 1.1,
	}, []string{"method", "route"})

	// GAUGE: sobe e desce.
	inFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Requisições sendo atendidas agora.",
	})

	// SUMMARY: quantis calculados NO CLIENTE. Não dá para agregar entre
	// réplicas! (exercício 03 troca por histogram)
	backendDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name:       "backend_call_duration_seconds",
		Help:       "Latência das chamadas ao backend de estoque.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	// ─────────────── RUINS (exercícios 01 e 04) ───────────────

	// RUIM: camelCase e counter sem _total.
	requestsCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "requestsCount",
		Help: "Checkouts realizados.",
	})
	// RUIM: milissegundos em vez de segundos (e abreviado).
	checkoutLatencyMs = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "checkout_latency_ms",
		Help:    "Latência do checkout em milissegundos.",
		Buckets: []float64{50, 100, 250, 500, 1000},
	})
	// RUIM: gauge usado como counter (só faz Inc) e com _total.
	ordersProcessed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "orders_processed_total",
		Help: "Pedidos processados.",
	})
	// RUIM: unidade abreviada e não-base (kb em vez de bytes).
	cacheSizeKb = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cache_size_kb",
		Help: "Tamanho do cache em kilobytes.",
	})
	// RUIM: sem HELP.
	cartItems = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cart_items_total",
	})
	// RUIM (e o promtool NÃO pega!): bomba de cardinalidade, um label por usuário.
	requestsByUser = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_by_user_total",
		Help: "Requisições por usuário.",
	}, []string{"user_id"})
)

func init() {
	good.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpRequests, httpDuration, inFlight, backendDuration,
	)
	bad.MustRegister(requestsCount, checkoutLatencyMs, ordersProcessed, cacheSizeKb, cartItems, requestsByUser)
}

var latencyFactor = 1.0 // BACKEND_LATENCY_FACTOR: réplica "b" é mais lenta

func traceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sleepMs(lo, hi float64) { time.Sleep(time.Duration((lo + mrand.Float64()*(hi-lo)) * float64(time.Millisecond))) }

// processPayment: função de negócio SEM instrumentação (exercício 06).
func processPayment() error {
	sleepMs(5, 40)
	if mrand.Float64() < 0.10 {
		return errDeclined
	}
	return nil
}

type declined struct{}

func (declined) Error() string { return "card declined" }

var errDeclined error = declined{}

// syncInventory: worker em background que chama o backend de estoque o tempo
// todo (independente do tráfego HTTP). Alimenta o summary backend_call_duration_seconds.
func syncInventory() {
	for {
		callBackend()
		time.Sleep(20 * time.Millisecond)
	}
}

func callBackend() {
	start := time.Now()
	sleepMs(10*latencyFactor, 50*latencyFactor)
	backendDuration.Observe(time.Since(start).Seconds())
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(c int) { r.code = c; r.ResponseWriter.WriteHeader(c) }

// instrument: middleware RED (Rate, Errors, Duration) + exemplar com trace_id.
func instrument(route string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inFlight.Inc()
		defer inFlight.Dec()
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: 200}
		h(rec, r)
		httpRequests.WithLabelValues(r.Method, route, strconv.Itoa(rec.code)).Inc()
		httpDuration.WithLabelValues(r.Method, route).(prometheus.ExemplarObserver).
			ObserveWithExemplar(time.Since(start).Seconds(), prometheus.Labels{"trace_id": traceID()})
	}
}

func items(w http.ResponseWriter, r *http.Request) {
	sleepMs(5, 50)
	cacheSizeKb.Set(float64(1024 + mrand.IntN(512)))
	_, _ = w.Write([]byte("items ok\n"))
}

func checkout(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestsByUser.WithLabelValues(r.Header.Get("X-User-Id")).Inc()
	cartItems.Add(float64(1 + mrand.IntN(4)))
	if mrand.Float64() < 0.07 {
		sleepMs(350, 600) // cauda lenta: ~7% acima de 300ms
	} else {
		sleepMs(20, 200)
	}
	if mrand.Float64() < 0.05 {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom\n"))
		return
	}
	if err := processPayment(); err != nil {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = w.Write([]byte(err.Error() + "\n"))
		return
	}
	requestsCount.Inc()
	ordersProcessed.Inc()
	checkoutLatencyMs.Observe(float64(time.Since(start).Milliseconds()))
	_, _ = w.Write([]byte("checkout ok\n"))
}

func main() {
	if f, err := strconv.ParseFloat(os.Getenv("BACKEND_LATENCY_FACTOR"), 64); err == nil {
		latencyFactor = f
	}
	go syncInventory()
	opts := promhttp.HandlerOpts{EnableOpenMetrics: true, EnableOpenMetricsTextCreatedSamples: true}
	http.Handle("/metrics", promhttp.HandlerFor(prometheus.Gatherers{good, bad}, opts))
	http.Handle("/metrics/good", promhttp.HandlerFor(good, opts))
	http.Handle("/metrics/bad", promhttp.HandlerFor(bad, opts))
	http.HandleFunc("/api/items", instrument("/api/items", items))
	http.HandleFunc("/api/checkout", instrument("/api/checkout", checkout))
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	log.Println("app-go ouvindo em :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
