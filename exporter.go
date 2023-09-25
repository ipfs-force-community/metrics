package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"contrib.go.opencensus.io/exporter/graphite"
	"contrib.go.opencensus.io/exporter/prometheus"
	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	promclient "github.com/prometheus/client_golang/prometheus"
	"go.opencensus.io/stats/view"
	octrace "go.opencensus.io/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/bridge/opencensus"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
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

		<-ctx.Done()
		log.Info("context done")

		view.UnregisterExporter(pe)
		if err := srv.Shutdown(context.TODO()); err != nil {
			log.Errorf("shutting down prometheus server failed: %s", err)
		}
	}()

	log.Info("Start prometheus exporter server ", lst.Addr())
	return srv.Serve(manet.NetListener(lst))
}

func RegisterGraphiteExporter(ctx context.Context, cfg *MetricsGraphiteExporterConfig) error {
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

	go func() {
		<-ctx.Done()
		log.Info("context done")

		view.UnregisterExporter(exporter)
	}()

	return nil
}

func SetupMetrics(ctx context.Context, cfg *MetricsConfig) error {
	if cfg.Enabled {
		switch cfg.Exporter.Type {
		case ETPrometheus:
			go func() {
				if err := RegisterPrometheusExporter(ctx, cfg.Exporter.Prometheus); err != nil {
					log.Errorf("failed to register prometheus exporter err: %v", err)
				}
				log.Infof("prometheus exporter server graceful shutdown successful")
			}()

		case ETGraphite:
			if err := RegisterGraphiteExporter(ctx, cfg.Exporter.Graphite); err != nil {
				log.Errorf("failed to register graphite exporter: %v", err)
			}
		default:
			log.Warnf("invalid exporter type: %s", cfg.Exporter.Type)
		}
	}

	return nil
}

// SetupJaegerTracing setups the jaeger endpoint and names the
// tracer.
func SetupJaegerTracing(serviceName string, cfg *TraceConfig) (*tracesdk.TracerProvider, error) {
	if !cfg.JaegerTracingEnabled {
		return nil, nil
	}

	if len(cfg.ServerName) != 0 {
		serviceName = cfg.ServerName
	}

	je, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(cfg.JaegerEndpoint)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(je),
		// Record information about this application in an Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
		tracesdk.WithSampler(tracesdk.TraceIDRatioBased(cfg.ProbabilitySampler)),
	)

	otel.SetTracerProvider(tp)
	tracer := tp.Tracer(serviceName)
	octrace.DefaultTracer = opencensus.NewTracer(tracer)

	return tp, err
}

func ShutdownJaeger(ctx context.Context, je *tracesdk.TracerProvider) error {
	if err := je.ForceFlush(ctx); err != nil {
		log.Warnf("failed to flush jaeger: %w", err)
	}
	return je.Shutdown(ctx)
}
