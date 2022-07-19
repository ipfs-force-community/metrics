package metrics

type ExporterType string

const (
	ETPrometheus ExporterType = "prometheus"
	ETGraphite   ExporterType = "graphite"
)

type TraceConfig struct {
	JaegerTracingEnabled bool    `json:"jaegerTracingEnabled"`
	ProbabilitySampler   float64 `json:"probabilitySampler"`
	JaegerEndpoint       string  `json:"jaegerEndpoint"`
	ServerName           string  `json:"servername"`
}

func DefaultTraceConfig() *TraceConfig {
	return &TraceConfig{
		JaegerTracingEnabled: false,
		JaegerEndpoint:       "localhost:6831",
		ProbabilitySampler:   1.0,
		ServerName:           "",
	}
}

type MetricsPrometheusExporterConfig struct {
	RegistryType    string `json:"registryType"`
	Namespace       string `json:"namespace"`
	EndPoint        string `json:"endPoint"`
	Path            string `json:"path"`
	ReportingPeriod string `json:"reportingPeriod"`
}

func newMetricsPrometheusExporterConfig() *MetricsPrometheusExporterConfig {
	return &MetricsPrometheusExporterConfig{
		RegistryType:    "define",
		Namespace:       "",
		EndPoint:        "/ip4/0.0.0.0/tcp/4568",
		Path:            "/debug/metrics",
		ReportingPeriod: "10s",
	}
}

type MetricsGraphiteExporterConfig struct {
	Namespace       string `json:"namespace"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	ReportingPeriod string `json:"reportingPeriod"`
}

func newMetricsGraphiteExporterConfig() *MetricsGraphiteExporterConfig {
	return &MetricsGraphiteExporterConfig{
		Namespace:       "",
		Host:            "127.0.0.1",
		Port:            4568,
		ReportingPeriod: "10s",
	}
}

type MetricsExporterConfig struct {
	Type ExporterType `json:"type"`

	Prometheus *MetricsPrometheusExporterConfig `json:"prometheus"`
	Graphite   *MetricsGraphiteExporterConfig   `json:"graphite"`
}

func newDefaultMetricsExporterConfig() *MetricsExporterConfig {
	return &MetricsExporterConfig{
		Type: ETPrometheus,

		Prometheus: newMetricsPrometheusExporterConfig(),
		Graphite:   newMetricsGraphiteExporterConfig(),
	}
}

type MetricsConfig struct {
	Enabled  bool                   `json:"enabled"`
	Exporter *MetricsExporterConfig `json:"exporter"`
}

func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:  false,
		Exporter: newDefaultMetricsExporterConfig(),
	}
}
