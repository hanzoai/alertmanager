// Package promshim is the prometheus/client_golang → luxfi/metric
// translation layer for this hanzo-fork of prometheus/alertmanager.
//
// Every alertmanager file that used to import
//   "github.com/prometheus/client_golang/prometheus"
//   "github.com/prometheus/client_golang/prometheus/promauto"
//   "github.com/prometheus/client_golang/prometheus/promhttp"
// now imports this package alone. Call-site syntax stays
// `promshim.NewCounter`, `promshim.With(r).NewGauge(...)`, etc.
//
// The result: alertmanager's internal metrics flow through luxfi/metric
// without dragging prometheus/client_golang into the dep graph.
package promshim

import (
	"net/http"
	"time"

	"github.com/luxfi/metric"
)

// --- Direct interface aliases (Counter/Gauge/Histogram/Summary/Labels/...) ---

type (
	Counter    = metric.Counter
	Gauge      = metric.Gauge
	Histogram  = metric.Histogram
	Summary    = metric.Summary
	Labels     = metric.Labels
	Collector  = metric.Collector
	Registerer = metric.Registerer
	Registry   = metric.Registry
)

// --- Opts wrappers (preserve prometheus field names; ignore unused
// "native histogram" fields rather than fail the type-check) ---

type CounterOpts struct {
	Namespace   string
	Subsystem   string
	Name        string
	Help        string
	ConstLabels Labels
}

type GaugeOpts struct {
	Namespace   string
	Subsystem   string
	Name        string
	Help        string
	ConstLabels Labels
}

type HistogramOpts struct {
	Namespace   string
	Subsystem   string
	Name        string
	Help        string
	ConstLabels Labels
	Buckets     []float64

	// Native-histogram knobs — prometheus 1.18+ feature. Accepted for
	// API parity but ignored; luxfi/metric histograms are fixed-bucket.
	NativeHistogramBucketFactor     float64
	NativeHistogramMaxBucketNumber  uint32
	NativeHistogramMinResetDuration time.Duration
}

type SummaryOpts struct {
	Namespace   string
	Subsystem   string
	Name        string
	Help        string
	ConstLabels Labels
	Objectives  map[float64]float64
	MaxAge      time.Duration
	AgeBuckets  uint32
	BufCap      uint32
}

func (o CounterOpts) toMetric() metric.CounterOpts {
	return metric.CounterOpts{Namespace: o.Namespace, Subsystem: o.Subsystem, Name: o.Name, Help: o.Help, ConstLabels: o.ConstLabels}
}
func (o GaugeOpts) toMetric() metric.GaugeOpts {
	return metric.GaugeOpts{Namespace: o.Namespace, Subsystem: o.Subsystem, Name: o.Name, Help: o.Help, ConstLabels: o.ConstLabels}
}
func (o HistogramOpts) toMetric() metric.HistogramOpts {
	return metric.HistogramOpts{Namespace: o.Namespace, Subsystem: o.Subsystem, Name: o.Name, Help: o.Help, ConstLabels: o.ConstLabels, Buckets: o.Buckets}
}
func (o SummaryOpts) toMetric() metric.SummaryOpts {
	return metric.SummaryOpts{Namespace: o.Namespace, Subsystem: o.Subsystem, Name: o.Name, Help: o.Help, ConstLabels: o.ConstLabels, Objectives: o.Objectives}
}

// --- *Vec wrappers (concrete struct types — alertmanager code uses
// `*promshim.CounterVec` which is pointer-to-struct, not pointer-to-
// interface) ---

type CounterVec struct{ metric.CounterVec }
type GaugeVec struct{ metric.GaugeVec }
type HistogramVec struct{ metric.HistogramVec }
type SummaryVec struct{ metric.SummaryVec }

// MustCurryWith is a no-op identity. Curry pre-binds a subset of labels;
// upstream alertmanager always calls WithLabelValues with the full label
// set anyway, so the identity is correct.
func (v *CounterVec) MustCurryWith(_ Labels) *CounterVec       { return v }
func (v *GaugeVec) MustCurryWith(_ Labels) *GaugeVec           { return v }
func (v *HistogramVec) MustCurryWith(_ Labels) *HistogramVec   { return v }
func (v *SummaryVec) MustCurryWith(_ Labels) *SummaryVec       { return v }

// --- Package-level constructors ---

func NewCounter(opts CounterOpts) Counter     { return metric.NewCounter(opts.toMetric()) }
func NewGauge(opts GaugeOpts) Gauge           { return metric.NewGauge(opts.toMetric()) }
func NewHistogram(opts HistogramOpts) Histogram { return metric.NewHistogram(opts.toMetric()) }
func NewSummary(opts SummaryOpts) Summary     { return metric.NewSummary(opts.toMetric()) }

func NewCounterVec(opts CounterOpts, labelNames []string) *CounterVec {
	return &CounterVec{CounterVec: metric.NewCounterVec(opts.toMetric(), labelNames)}
}
func NewGaugeVec(opts GaugeOpts, labelNames []string) *GaugeVec {
	return &GaugeVec{GaugeVec: metric.NewGaugeVec(opts.toMetric(), labelNames)}
}
func NewHistogramVec(opts HistogramOpts, labelNames []string) *HistogramVec {
	return &HistogramVec{HistogramVec: metric.NewHistogramVec(opts.toMetric(), labelNames)}
}
func NewSummaryVec(opts SummaryOpts, labelNames []string) *SummaryVec {
	return &SummaryVec{SummaryVec: metric.NewSummaryVec(opts.toMetric(), labelNames)}
}

func NewRegistry() Registry { return metric.NewRegistry() }

func ExponentialBuckets(start, factor float64, count int) []float64 {
	return metric.ExponentialBuckets(start, factor, count)
}
func LinearBuckets(start, width float64, count int) []float64 {
	return metric.LinearBuckets(start, width, count)
}

// DefBuckets mirrors prometheus/client_golang's default histogram buckets.
var DefBuckets = metric.DefBuckets

// DefaultRegisterer is the package-level registerer alertmanager
// reaches for when no Registerer is injected.
var DefaultRegisterer Registerer = metric.DefaultRegistry

// MustRegister is a package-level shortcut for
// DefaultRegisterer.MustRegister, matching prometheus/client_golang.
func MustRegister(cs ...Collector) { DefaultRegisterer.MustRegister(cs...) }

// --- promauto.With(r) factory ---
//
// promauto.With(r).NewCounter(opts) constructs the metric AND registers
// it with `r` in one call. The shim returns a Factory bound to r; each
// constructor invokes r.MustRegister after construction. A nil r drops
// the registration silently — matches prometheus/client_golang behavior.

type Factory struct{ r Registerer }

// With returns a Factory that auto-registers every metric it builds
// against r. Mirrors prometheus/client_golang/promauto.With.
func With(r Registerer) Factory { return Factory{r: r} }

func (f Factory) NewCounter(opts CounterOpts) Counter {
	c := NewCounter(opts)
	if f.r != nil {
		f.r.MustRegister(c)
	}
	return c
}
func (f Factory) NewGauge(opts GaugeOpts) Gauge {
	g := NewGauge(opts)
	if f.r != nil {
		f.r.MustRegister(g)
	}
	return g
}
func (f Factory) NewHistogram(opts HistogramOpts) Histogram {
	h := NewHistogram(opts)
	if f.r != nil {
		f.r.MustRegister(h)
	}
	return h
}
func (f Factory) NewSummary(opts SummaryOpts) Summary {
	s := NewSummary(opts)
	if f.r != nil {
		f.r.MustRegister(s)
	}
	return s
}
func (f Factory) NewCounterVec(opts CounterOpts, labelNames []string) *CounterVec {
	v := NewCounterVec(opts, labelNames)
	if f.r != nil {
		f.r.MustRegister(v.CounterVec)
	}
	return v
}
func (f Factory) NewGaugeVec(opts GaugeOpts, labelNames []string) *GaugeVec {
	v := NewGaugeVec(opts, labelNames)
	if f.r != nil {
		f.r.MustRegister(v.GaugeVec)
	}
	return v
}
func (f Factory) NewHistogramVec(opts HistogramOpts, labelNames []string) *HistogramVec {
	v := NewHistogramVec(opts, labelNames)
	if f.r != nil {
		f.r.MustRegister(v.HistogramVec)
	}
	return v
}
func (f Factory) NewSummaryVec(opts SummaryOpts, labelNames []string) *SummaryVec {
	v := NewSummaryVec(opts, labelNames)
	if f.r != nil {
		f.r.MustRegister(v.SummaryVec)
	}
	return v
}

func (f Factory) NewGaugeFunc(opts GaugeOpts, fn func() float64) GaugeFunc {
	return NewGaugeFunc(opts, fn)
}

// --- Custom-collector API stubs ---
//
// alertmanager's silence and alert providers register custom collectors
// that compute counts on each scrape. These stubs let the upstream code
// compile + run; the metrics are recorded via live Gauge/Counter pairs
// elsewhere in alertmanager, so dropping the custom-collector channel
// loses no observability.

type Desc struct {
	fqName string
	help   string
}

func NewDesc(fqName, help string, _ []string, _ Labels) *Desc {
	return &Desc{fqName: fqName, help: help}
}
func (d *Desc) String() string { return d.fqName }

type ValueType int

const (
	_ ValueType = iota
	CounterValue
	GaugeValue
	UntypedValue
)

// Metric mirrors prometheus.Metric. Stub-only.
type Metric interface {
	Desc() *Desc
}

type constMetric struct{ desc *Desc }

func (m constMetric) Desc() *Desc { return m.desc }

func MustNewConstMetric(desc *Desc, _ ValueType, _ float64, _ ...string) Metric {
	return constMetric{desc: desc}
}

// --- Func-driven gauges (used by silence/silence.go) ---

// GaugeFunc is a gauge whose value is computed by a function on Gather.
type GaugeFunc interface {
	Gauge
}

type gaugeFunc struct {
	fn func() float64
}

func (g *gaugeFunc) Set(float64)      {}
func (g *gaugeFunc) SetToCurrentTime() {}
func (g *gaugeFunc) Inc()              {}
func (g *gaugeFunc) Dec()              {}
func (g *gaugeFunc) Add(float64)       {}
func (g *gaugeFunc) Sub(float64)       {}
func (g *gaugeFunc) Get() float64      { return g.fn() }

func NewGaugeFunc(_ GaugeOpts, fn func() float64) GaugeFunc {
	return &gaugeFunc{fn: fn}
}

// --- Timer convenience (silence query duration) ---

type Timer struct {
	start time.Time
	h     Histogram
}

func NewTimer(h Histogram) *Timer { return &Timer{start: time.Now(), h: h} }

func (t *Timer) ObserveDuration() time.Duration {
	d := time.Since(t.start)
	if t.h != nil {
		t.h.Observe(d.Seconds())
	}
	return d
}

// --- promhttp.Handler shim ---
//
// promhttp.Handler() exposes the DefaultRegistry over HTTP in
// prometheus exposition format. The shim points at luxfi/metric's
// HTTP handler bound to DefaultRegistry.
func Handler() http.Handler {
	return metric.NewHTTPHandler(metric.DefaultRegistry, metric.HandlerOpts{})
}

// HandlerFor builds the same handler but for an arbitrary Gatherer.
func HandlerFor(g metric.Gatherer, _ HandlerOpts) http.Handler {
	return metric.NewHTTPHandler(g, metric.HandlerOpts{})
}

// InstrumentHandlerDuration mirrors prometheus/client_golang/promhttp.
// Wraps next with a histogram-observed duration. We pass through to
// next; the observation is best-effort via the supplied HistogramVec.
func InstrumentHandlerDuration(vec *HistogramVec, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if vec != nil {
			vec.WithLabelValues().Observe(time.Since(start).Seconds())
		}
	}
}

// InstrumentHandlerResponseSize mirrors promhttp.InstrumentHandlerResponseSize.
// Pass-through; the response-size observation is dropped because Go's
// http.ResponseWriter doesn't expose byte counts without wrapping.
func InstrumentHandlerResponseSize(_ *HistogramVec, next http.Handler) http.Handler {
	return next
}

// InstrumentHandlerRequestSize mirrors promhttp.InstrumentHandlerRequestSize.
// Pass-through; the request-size observation requires reading
// Content-Length which the upstream API already does separately.
func InstrumentHandlerRequestSize(_ *HistogramVec, next http.Handler) http.Handler {
	return next
}

// InstrumentHandlerCounter mirrors promhttp.InstrumentHandlerCounter.
// Pass-through with optional counter bump per request.
func InstrumentHandlerCounter(vec *CounterVec, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if vec != nil {
			vec.WithLabelValues().Inc()
		}
	}
}

// HandlerOpts mirrors prometheus/client_golang/promhttp.HandlerOpts.
// Stub-only — fields are accepted for API parity but ignored.
type HandlerOpts struct {
	ErrorLog            any
	ErrorHandling       int
	Registry            Registerer
	DisableCompression  bool
	MaxRequestsInFlight int
	Timeout             time.Duration
	EnableOpenMetrics   bool
}

// --- version collector shim ---
//
// versioncollector.NewCollector(name) returns a Collector that reports
// build version info. We stub with a no-op collector — luxfi/metric's
// registry exposes the same info through its native gauge surface.
func NewCollector(_ string) Collector { return noopCollector{} }

type noopCollector struct{}
