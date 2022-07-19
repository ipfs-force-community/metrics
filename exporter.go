package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"contrib.go.opencensus.io/exporter/graphite"
	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

var log = logging.Logger("metrics")

type RegistryType string

const (
	RTDefault RegistryType = "default"
	RTDefine  RegistryType = "define"
)

// RegisterPrometheusExporter register the prometheus exporter
func RegisterPrometheusExporter(ctx context.Context, cfg *MetricsPrometheusExporterConfig) error {
	// validate config values and marshal to types
	reportPeriod, err := time.ParseDuration(cfg.ReportingPeriod)
	if err != nil {
		return err
	}

	promma, err := ma.NewMultiaddr(cfg.EndPoint)
	if err != nil {
		return err
	}

	lst, err := manet.Listen(promma)
	if err != nil {
		return fmt.Errorf("could not listen: %w", err)
	}

	// setup prometheus
	var registry *promclient.Registry
	var ok bool
	switch RegistryType(cfg.RegistryType) {
	case RTDefault:
		// Prometheus globals are exposed as interfaces, but the prometheus
		// OpenCensus exporter expects a concrete *Registry. The concrete type of
		// the globals are actually *Registry, so we downcast them, staying
		// defensive in case things change under the hood.
		registry, ok = promclient.DefaultRegisterer.(*promclient.Registry)
		if !ok {
			return fmt.Errorf("failed to export default prometheus registry; some metrics will be unavailable; unexpected type: %T", promclient.DefaultRegisterer)
		}
	case RTDefine:
		// The metrics of OpenCensus in the same process will be automatically
		// registered to the custom registry of Prometheus, so no additional
		// registration action is required
		registry = promclient.NewRegistry()
	default:
		return fmt.Errorf("wrong registry type: %s", cfg.RegistryType)
	}

	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: cfg.Namespace,
		Registry:  registry,
	})
	if err != nil {
		return fmt.Errorf("could not create the prometheus stats exporter: %w", err)
	}

	view.RegisterExporter(pe)
	view.SetReportingPeriod(reportPeriod)

	mux := http.NewServeMux()
	mux.Handle(cfg.Path, pe)
	srv := &http.Server{
		Handler: mux,
	}

	go func() {
		select {
		case <-ctx.Done():
		}

		if err := srv.Shutdown(context.TODO()); err != nil {
			log.Errorf("shutting down prometheus server failed: %s", err)
		}
	}()

	log.Info("Start prometheus exporter server ", lst.Addr())
	return srv.Serve(manet.NetListener(lst))
}

func RegisterGraphiteExporter(cfg *MetricsGraphiteExporterConfig) error {
	exporter, err := graphite.NewExporter(graphite.Options{Namespace: cfg.Namespace, Host: cfg.Host, Port: cfg.Port})
	if err != nil {
		return fmt.Errorf("failed to create graphite exporter: %w", err)
	}

	view.RegisterExporter(exporter)

	reportPeriod, err := time.ParseDuration(cfg.ReportingPeriod)
	if err != nil {
		return err
	}
	view.SetReportingPeriod(reportPeriod)

	return nil
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

func UnregisterJaeger(exp *jaeger.Exporter) {
	exp.Flush()
	trace.UnregisterExporter(exp)
}
