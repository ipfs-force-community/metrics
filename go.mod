module github.com/ipfs-force-community/metrics

go 1.16

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.1
	contrib.go.opencensus.io/exporter/prometheus v0.3.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/go-redis/redis/v8 v8.11.0
	github.com/go-redis/redis_rate/v9 v9.1.1
	github.com/gorilla/mux v1.7.3
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/prometheus/client_golang v1.11.0
	github.com/whyrusleeping/go-logging v0.0.1
	go.opencensus.io v0.23.0
)
