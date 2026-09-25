package main

// Núcleo do gerador de métricas fake do PromQL Lab.
//
// Cada função PromQL tem um arquivo functions/<fn>/setup/scenario.go.
// No build, todos esses arquivos são copiados para este pacote (package main)
// e se registram via init() chamando Register(...).
//
// Regras para um scenario.go:
//   - package main, SEM identificadores top-level (só func init()).
//   - Nomes de métricas começam com "<função>_" (ex.: rate_http_requests_total).

import (
	"log"
	"math"
	"math/rand/v2"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Scenario descreve os dados fake de uma lição.
type Scenario struct {
	Name  string                                  // nome da função PromQL (ex.: "rate")
	Setup func(reg prometheus.Registerer)         // registra coletores / inicia goroutines
}

var (
	scenariosMu sync.Mutex
	scenarios   = map[string]Scenario{}
	// StartTime é quando o gerador subiu — útil para cenários baseados em tempo.
	StartTime = time.Now()
)

// Register é chamado pelo init() de cada scenario.go.
func Register(s Scenario) {
	scenariosMu.Lock()
	defer scenariosMu.Unlock()
	if _, dup := scenarios[s.Name]; dup {
		log.Printf("WARN: scenario %q registrado duas vezes, ignorando", s.Name)
		return
	}
	scenarios[s.Name] = s
}

func sortedScenarios() []Scenario {
	scenariosMu.Lock()
	defer scenariosMu.Unlock()
	out := make([]Scenario, 0, len(scenarios))
	for _, s := range scenarios {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Elapsed retorna segundos desde que o gerador subiu.
func Elapsed() float64 { return time.Since(StartTime).Seconds() }

// Sample é um ponto calculado na hora do scrape.
type Sample struct {
	Labels  []string  // valores na mesma ordem de LabelNames
	Value   float64
	Created time.Time // opcional: start timestamp (created) de counters
}

// FuncCollector calcula as amostras sob demanda, a cada scrape.
// Ideal para dados determinísticos (ondas, rampas, resets programados,
// séries que somem/aparecem...). Retornar nil faz a série "sumir".
type FuncCollector struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
	fn        func(now time.Time) []Sample
}

// NewFunc cria um coletor calculado.
//   kind: prometheus.GaugeValue ou prometheus.CounterValue (ou UntypedValue)
func NewFunc(name, help string, kind prometheus.ValueType, labelNames []string, fn func(now time.Time) []Sample) *FuncCollector {
	return &FuncCollector{
		desc:      prometheus.NewDesc(name, help, labelNames, nil),
		valueType: kind,
		fn:        fn,
	}
}

func (c *FuncCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *FuncCollector) Collect(ch chan<- prometheus.Metric) {
	for _, s := range c.fn(time.Now()) {
		var (
			m   prometheus.Metric
			err error
		)
		if !s.Created.IsZero() && c.valueType == prometheus.CounterValue {
			m, err = prometheus.NewConstMetricWithCreatedTimestamp(c.desc, c.valueType, s.Value, s.Created, s.Labels...)
		} else {
			m, err = prometheus.NewConstMetric(c.desc, c.valueType, s.Value, s.Labels...)
		}
		if err != nil {
			log.Printf("collect %v: %v", c.desc, err)
			continue
		}
		ch <- m
	}
}

// ---------- helpers matemáticos para gerar formas de onda ----------

// Wave é uma senóide: base + amp*sin(2π t/period + phase).
func Wave(t, period, base, amp, phase float64) float64 {
	return base + amp*math.Sin(2*math.Pi*t/period+phase)
}

// Saw é uma dente-de-serra de 0 até 1 com o período dado.
func Saw(t, period float64) float64 { return math.Mod(t, period) / period }

// Square alterna entre 0 e 1 (1 na primeira metade do período).
func Square(t, period float64) float64 {
	if math.Mod(t, period) < period/2 {
		return 1
	}
	return 0
}

// Noise retorna ruído uniforme em [-amp, +amp].
func Noise(amp float64) float64 { return (rand.Float64()*2 - 1) * amp }

// Every executa fn a cada intervalo, numa goroutine (para contadores/histogramas "vivos").
func Every(d time.Duration, fn func()) {
	go func() {
		t := time.NewTicker(d)
		defer t.Stop()
		for range t.C {
			fn()
		}
	}()
}
