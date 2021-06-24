package metrics

import (
	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	prom "github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"net/http"
	"time"
)

// RegisterPrometheusEndpoint registers and serves prometheus metrics
func RegisterPrometheusEndpoint(cfg *MetricsConfig) error {
	if !cfg.PrometheusEnabled {
		return nil
	}

	// validate config values and marshal to types
	interval, err := time.ParseDuration(cfg.ReportInterval)
	if err != nil {
		return err
	}

	promma, err := ma.NewMultiaddr(cfg.PrometheusEndpoint)
	if err != nil {
		return err
	}

	_, promAddr, err := manet.DialArgs(promma) // nolint
	if err != nil {
		return err
	}

	// setup prometheus
	registry := prom.NewRegistry()
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "filecoin",
		Registry:  registry,
	})
	if err != nil {
		return err
	}

	view.RegisterExporter(pe)
	view.SetReportingPeriod(interval)

	mux := http.NewServeMux()
	mux.Handle("/metrics", pe)
	return http.ListenAndServe(promAddr, mux)
}

// RegisterJaeger registers the jaeger endpoint with opencensus and names the
// tracer `name`.
func RegisterJaeger(name string, cfg *TraceConfig) (*jaeger.Exporter, error) {
	if !cfg.JaegerTracingEnabled {
		return nil, nil
	}

	if len(cfg.ServerName) != 0 {
		name = cfg.ServerName
	}

	je, err := jaeger.NewExporter(jaeger.Options{
		AgentEndpoint: cfg.JaegerEndpoint,
		Process: jaeger.Process{
			ServiceName: name,
		},
	})
	if err != nil {
		return nil, err
	}

	trace.RegisterExporter(je)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.ProbabilitySampler(cfg.ProbabilitySampler)})

	return je, err
}
