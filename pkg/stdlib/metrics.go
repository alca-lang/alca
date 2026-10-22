// Package stdlib provides enterprise-grade standard libraries for the Alca programming language.
package stdlib

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
	"github.com/alca-lang/alca/pkg/sandbox"
)

// MetricType defines the category of a recorded metric.
type MetricType string

const (
	MetricCounter   MetricType = "counter"
	MetricGauge     MetricType = "gauge"
	MetricHistogram MetricType = "histogram"
)

type metricEntry struct {
	name       string
	help       string
	mType      MetricType
	mu         sync.RWMutex
	value      float64
	buckets    []float64
	bucketHits map[float64]int64
	count      int64
	sum        float64
}

// MetricsRegistry maintains thread-safe application metrics for Prometheus scraping.
type MetricsRegistry struct {
	mu      sync.RWMutex
	metrics map[string]*metricEntry
}

var globalRegistry = &MetricsRegistry{
	metrics: make(map[string]*metricEntry),
}

// GetRegistry returns the global application metrics registry.
func GetRegistry() *MetricsRegistry {
	return globalRegistry
}

// ExportText returns Prometheus/OpenMetrics exposition format string.
func (r *MetricsRegistry) ExportText() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sb strings.Builder
	keys := make([]string, 0, len(r.metrics))
	for k := range r.metrics {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		m := r.metrics[k]
		m.mu.RLock()

		sb.WriteString(fmt.Sprintf("# HELP %s %s\n", m.name, m.help))
		sb.WriteString(fmt.Sprintf("# TYPE %s %s\n", m.name, string(m.mType)))

		switch m.mType {
		case MetricCounter, MetricGauge:
			sb.WriteString(fmt.Sprintf("%s %g\n", m.name, m.value))

		case MetricHistogram:
			cumCount := int64(0)
			for _, b := range m.buckets {
				cumCount += m.bucketHits[b]
				sb.WriteString(fmt.Sprintf("%s_bucket{le=\"%g\"} %d\n", m.name, b, cumCount))
			}
			sb.WriteString(fmt.Sprintf("%s_bucket{le=\"+Inf\"} %d\n", m.name, m.count))
			sb.WriteString(fmt.Sprintf("%s_sum %g\n", m.name, m.sum))
			sb.WriteString(fmt.Sprintf("%s_count %d\n", m.name, m.count))
		}

		m.mu.RUnlock()
	}

	return sb.String()
}

// RegisterMetricsBuiltins registers Prometheus observability into Alca environment.
func RegisterMetricsBuiltins(env *interpreter.Environment) {
	metricsModule := interpreter.NewMap()

	// metrics.counter(name, help)
	metricsModule.Set("counter", &interpreter.BuiltinFunction{
		Name: "metrics.counter",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "metrics.counter requires name")
			}
			name := interpreter.CoerceToString(args[0])
			help := ""
			if len(args) > 1 {
				help = interpreter.CoerceToString(args[1])
			}

			globalRegistry.mu.Lock()
			entry, ok := globalRegistry.metrics[name]
			if !ok {
				entry = &metricEntry{
					name:  name,
					help:  help,
					mType: MetricCounter,
				}
				globalRegistry.metrics[name] = entry
			}
			globalRegistry.mu.Unlock()

			counterObj := interpreter.NewMap()
			counterObj.Set("name", &interpreter.StringValue{Val: name})
			counterObj.Set("inc", &interpreter.BuiltinFunction{
				Name: "inc",
				Fn: func(iArgs ...interpreter.Value) interpreter.Value {
					delta := 1.0
					if len(iArgs) > 0 {
						if iv, ok := iArgs[0].(*interpreter.IntegerValue); ok {
							delta = float64(iv.Val)
						} else if fv, ok := iArgs[0].(*interpreter.FloatValue); ok {
							delta = fv.Val
						}
					}
					entry.mu.Lock()
					entry.value += delta
					current := entry.value
					entry.mu.Unlock()
					return &interpreter.FloatValue{Val: current}
				},
			})
			counterObj.Set("get", &interpreter.BuiltinFunction{
				Name: "get",
				Fn: func(iArgs ...interpreter.Value) interpreter.Value {
					entry.mu.RLock()
					v := entry.value
					entry.mu.RUnlock()
					return &interpreter.FloatValue{Val: v}
				},
			})
			return counterObj
		},
	})

	// metrics.gauge(name, help)
	metricsModule.Set("gauge", &interpreter.BuiltinFunction{
		Name: "metrics.gauge",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "metrics.gauge requires name")
			}
			name := interpreter.CoerceToString(args[0])
			help := ""
			if len(args) > 1 {
				help = interpreter.CoerceToString(args[1])
			}

			globalRegistry.mu.Lock()
			entry, ok := globalRegistry.metrics[name]
			if !ok {
				entry = &metricEntry{
					name:  name,
					help:  help,
					mType: MetricGauge,
				}
				globalRegistry.metrics[name] = entry
			}
			globalRegistry.mu.Unlock()

			gaugeObj := interpreter.NewMap()
			gaugeObj.Set("name", &interpreter.StringValue{Val: name})
			gaugeObj.Set("set", &interpreter.BuiltinFunction{
				Name: "set",
				Fn: func(iArgs ...interpreter.Value) interpreter.Value {
					if len(iArgs) < 1 {
						return interpreter.NewError(400, "gauge.set requires value")
					}
					val := 0.0
					if iv, ok := iArgs[0].(*interpreter.IntegerValue); ok {
						val = float64(iv.Val)
					} else if fv, ok := iArgs[0].(*interpreter.FloatValue); ok {
						val = fv.Val
					}
					entry.mu.Lock()
					entry.value = val
					entry.mu.Unlock()
					return &interpreter.FloatValue{Val: val}
				},
			})
			gaugeObj.Set("inc", &interpreter.BuiltinFunction{
				Name: "inc",
				Fn: func(iArgs ...interpreter.Value) interpreter.Value {
					delta := 1.0
					if len(iArgs) > 0 {
						if iv, ok := iArgs[0].(*interpreter.IntegerValue); ok {
							delta = float64(iv.Val)
						} else if fv, ok := iArgs[0].(*interpreter.FloatValue); ok {
							delta = fv.Val
						}
					}
					entry.mu.Lock()
					entry.value += delta
					cur := entry.value
					entry.mu.Unlock()
					return &interpreter.FloatValue{Val: cur}
				},
			})
			gaugeObj.Set("dec", &interpreter.BuiltinFunction{
				Name: "dec",
				Fn: func(iArgs ...interpreter.Value) interpreter.Value {
					delta := 1.0
					if len(iArgs) > 0 {
						if iv, ok := iArgs[0].(*interpreter.IntegerValue); ok {
							delta = float64(iv.Val)
						} else if fv, ok := iArgs[0].(*interpreter.FloatValue); ok {
							delta = fv.Val
						}
					}
					entry.mu.Lock()
					entry.value -= delta
					cur := entry.value
					entry.mu.Unlock()
					return &interpreter.FloatValue{Val: cur}
				},
			})
			gaugeObj.Set("get", &interpreter.BuiltinFunction{
				Name: "get",
				Fn: func(iArgs ...interpreter.Value) interpreter.Value {
					entry.mu.RLock()
					v := entry.value
					entry.mu.RUnlock()
					return &interpreter.FloatValue{Val: v}
				},
			})
			return gaugeObj
		},
	})

	// metrics.histogram(name, help, buckets)
	metricsModule.Set("histogram", &interpreter.BuiltinFunction{
		Name: "metrics.histogram",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "metrics.histogram requires name")
			}
			name := interpreter.CoerceToString(args[0])
			help := ""
			if len(args) > 1 {
				help = interpreter.CoerceToString(args[1])
			}
			var buckets []float64
			if len(args) > 2 {
				if bl, ok := args[2].(*interpreter.ListValue); ok {
					for _, item := range bl.ElementsSlice() {
						if iv, ok := item.(*interpreter.IntegerValue); ok {
							buckets = append(buckets, float64(iv.Val))
						} else if fv, ok := item.(*interpreter.FloatValue); ok {
							buckets = append(buckets, fv.Val)
						}
					}
				}
			}
			if len(buckets) == 0 {
				buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
			}
			sort.Float64s(buckets)

			globalRegistry.mu.Lock()
			entry, ok := globalRegistry.metrics[name]
			if !ok {
				entry = &metricEntry{
					name:       name,
					help:       help,
					mType:      MetricHistogram,
					buckets:    buckets,
					bucketHits: make(map[float64]int64),
				}
				globalRegistry.metrics[name] = entry
			}
			globalRegistry.mu.Unlock()

			histObj := interpreter.NewMap()
			histObj.Set("name", &interpreter.StringValue{Val: name})
			histObj.Set("observe", &interpreter.BuiltinFunction{
				Name: "observe",
				Fn: func(iArgs ...interpreter.Value) interpreter.Value {
					if len(iArgs) < 1 {
						return interpreter.NewError(400, "histogram.observe requires value")
					}
					val := 0.0
					if iv, ok := iArgs[0].(*interpreter.IntegerValue); ok {
						val = float64(iv.Val)
					} else if fv, ok := iArgs[0].(*interpreter.FloatValue); ok {
						val = fv.Val
					}
					entry.mu.Lock()
					entry.count++
					entry.sum += val
					// Count the observation in the smallest bucket that contains
					// it; ExportText turns these per-bucket counts into the
					// cumulative Prometheus form.
					for _, b := range entry.buckets {
						if val <= b {
							entry.bucketHits[b]++
							break
						}
					}
					entry.mu.Unlock()
					return &interpreter.FloatValue{Val: val}
				},
			})
			return histObj
		},
	})

	// metrics.export() -> string
	metricsModule.Set("export", &interpreter.BuiltinFunction{
		Name: "metrics.export",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			return &interpreter.StringValue{Val: globalRegistry.ExportText()}
		},
	})

	// metrics.serve(port, path) -> starts HTTP endpoint in goroutine
	metricsModule.Set("serve", &interpreter.BuiltinFunction{
		Name: "metrics.serve",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			port := 9090
			if len(args) > 0 {
				if iv, ok := args[0].(*interpreter.IntegerValue); ok {
					port = int(iv.Val)
				}
			}
			path := "/metrics"
			if len(args) > 1 {
				path = interpreter.CoerceToString(args[1])
			}

			addr := ":" + strconv.Itoa(port)
			if err := sandbox.RequireNet(addr); err != nil {
				return handleIOError("metrics.serve", addr, err)
			}
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return handleIOError("metrics.serve", addr, err)
			}

			mux := http.NewServeMux()
			mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain; version=0.0.4")
				w.Write([]byte(globalRegistry.ExportText()))
			})

			server := &http.Server{
				Handler:      mux,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
			}

			go func() {
				_ = server.Serve(listener)
			}()

			boundPort := port
			if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
				boundPort = tcpAddr.Port
			}
			return &interpreter.StringValue{Val: fmt.Sprintf("http://localhost:%d%s", boundPort, path)}
		},
	})

	env.Set("metrics", metricsModule)
	env.Set("std.metrics", metricsModule)
}
