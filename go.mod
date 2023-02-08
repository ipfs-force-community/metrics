module github.com/ipfs-force-community/metrics

go 1.16

require (
	contrib.go.opencensus.io/exporter/graphite v0.0.0-20200424223504-26b90655e0ce
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	contrib.go.opencensus.io/exporter/prometheus v0.4.2
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/filecoin-project/venus v1.2.4
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-redis/redis/v7 v7.0.0-beta
	github.com/go-redis/redis_rate/v7 v7.0.1
	github.com/ipfs/go-log/v2 v2.5.0
	github.com/ipfs/go-metrics-interface v0.0.1
	github.com/multiformats/go-multiaddr v0.5.0
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/prometheus/statsd_exporter v0.23.0 // indirect
	github.com/whyrusleeping/go-logging v0.0.1
	go.opencensus.io v0.24.0
	go.uber.org/fx v1.15.0
	golang.org/x/sys v0.5.0 // indirect
)
