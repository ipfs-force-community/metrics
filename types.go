package metrics

type TraceConfig struct {
	JaegerTracingEnabled bool    `json:"jaegerTracingEnabled"`
	ProbabilitySampler   float64 `json:"probabilitySampler"`
	JaegerEndpoint       string  `json:"jaegerEndpoint"`
	ServerName           string  `json:"servername"`
}

type MetricsConfig struct {
	PrometheusEnabled  bool   `json:"prometheusEnabled"`
	ReportInterval     string `json:"reportInterval"`
	PrometheusEndpoint string `json:"prometheusEndpoint"`
}
