// GABARITO do app-go: todos os exercícios aplicados (01 a 06).
// Procure por "EX0x" para ver cada mudança.
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
	bad  = prometheus.NewRegistry() // agora só tem métricas corrigidas

	httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total de requisições HTTP atendidas.",
	}, []string{"method", "route", "code"})

	// EX02: buckets escolhidos para o SLO "95% do /api/checkout em até 300ms".
	// Precisa existir um bucket EXATAMENTE em 0.3; mais resolução perto do alvo.
	httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:                        "http_request_duration_seconds",
		Help:                        "Latência das requisições HTTP.",
		Buckets:                     []float64{0.025, 0.05, 0.1, 0.2, 0.3, 0.45, 0.6, 1, 2.5},
		NativeHistogramBucketFactor: 1.1,
	}, []string{"method", "route"})

	inFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Requisições sendo atendidas agora.",
	})

	// EX03: summary -> histogram (agregável entre réplicas com sum by (le)).
	backendDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "backend_call_duration_seconds",
		Help:    "Latência das chamadas ao backend de estoque.",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.5, 1},
	})

	// EX05: métrica _info (gauge com valor 1; a informação está nos labels).
	appInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "app_info",
		Help: "Metadados da aplicação (valor sempre 1).",
	}, []string{"version", "commit", "language"})

	// EX06: função de negócio instrumentada com counter + histogram.
	payments = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "payments_total",
		Help: "Pagamentos processados, por resultado.",
	}, []string{"result"})
	paymentDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "payment_duration_seconds",
		Help:    "Duração do processamento de pagamento.",
		Buckets: []float64{0.005, 0.01, 0.02, 0.03, 0.04, 0.05, 0.1},
	})

	// EX01: nomes corrigidos.
	checkouts = prometheus.NewCounter(prometheus.CounterOpts{ // era requestsCount
		Name: "checkouts_total",
		Help: "Checkouts realizados.",
	})
	checkoutDuration = prometheus.NewHistogram(prometheus.HistogramOpts{ // era checkout_latency_ms
		Name:    "checkout_duration_seconds",
		Help:    "Latência do checkout.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1},
	})
	ordersProcessed = prometheus.NewCounter(prometheus.CounterOpts{ // era um Gauge
		Name: "orders_processed_total",
		Help: "Pedidos processados.",
	})
	cacheSize = prometheus.NewGauge(prometheus.GaugeOpts{ // era cache_size_kb
		Name: "cache_size_bytes",
		Help: "Tamanho do cache.",
	})
	cartItems = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "cart_items_total",
		Help: "Itens adicionados ao carrinho.", // EX01: HELP adicionado
	})
	// EX04: user_id (ilimitado) -> plan (3 valores possíveis).
	checkoutsByPlan = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "checkouts_by_plan_total",
		Help: "Checkouts por plano do cliente.",
	}, []string{"plan"})
)

func init() {
	good.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		httpRequests, httpDuration, inFlight, backendDuration, appInfo, payments, paymentDuration,
	)
	bad.MustRegister(checkouts, checkoutDuration, ordersProcessed, cacheSize, cartItems, checkoutsByPlan)
	appInfo.WithLabelValues("1.4.2", "9f3c2ab", "go").Set(1)
}

var latencyFactor = 1.0

func traceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sleepMs(lo, hi float64) { time.Sleep(time.Duration((lo + mrand.Float64()*(hi-lo)) * float64(time.Millisecond))) }

func planOf(userID string) string {
	n, _ := strconv.Atoi(userID)
	return [...]string{"free", "pro", "enterprise"}[n%3]
}

// EX06: a função continua igual; a instrumentação fica num defer.
func processPayment() (err error) {
	start := time.Now()
	defer func() {
		paymentDuration.Observe(time.Since(start).Seconds())
		result := "success"
		if err != nil {
			result = "declined"
		}
		payments.WithLabelValues(result).Inc()
	}()
	sleepMs(5, 40)
	if mrand.Float64() < 0.10 {
		return errDeclined
	}
	return nil
}

type declined struct{}

func (declined) Error() string { return "card declined" }

var errDeclined error = declined{}

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
	cacheSize.Set(float64((1024 + mrand.IntN(512)) * 1024)) // EX01: bytes
	_, _ = w.Write([]byte("items ok\n"))
}

func checkout(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	checkoutsByPlan.WithLabelValues(planOf(r.Header.Get("X-User-Id"))).Inc()
	cartItems.Add(float64(1 + mrand.IntN(4)))
	if mrand.Float64() < 0.07 {
		sleepMs(350, 600)
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
	checkouts.Inc()
	ordersProcessed.Inc()
	checkoutDuration.Observe(time.Since(start).Seconds()) // EX01: segundos
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
	log.Println("app-go (gabarito) ouvindo em :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
