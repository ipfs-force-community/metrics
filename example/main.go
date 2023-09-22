package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/ipfs-force-community/metrics"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	"contrib.go.opencensus.io/exporter/prometheus"
)

func main() {
	ctx := context.Background()

	exporter, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		log.Fatal(err)
	}

	// example(ctx)
	testTimer(ctx)

	addr := ":9999"
	log.Printf("Serving at %s", addr)
	http.Handle("/metrics", exporter)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func example(ctx context.Context) { //nolint:deadcode,unused

	// Create measures. The program will record measures for the size of
	// processed videos and the number of videos marked as spam.
	var (
		videoCount = stats.Int64("example.com/measures/video_count", "number of processed videos", stats.UnitDimensionless)
		videoSize  = stats.Int64("example.com/measures/video_size", "size of processed video", stats.UnitBytes)
	)

	// Record some data points...
	keyMethod := tag.MustNewKey("method")
	keyStatus := tag.MustNewKey("status")

	// Create view to see the number of processed videos cumulatively.
	// Create view to see the amount of video processed
	// Subscribe will allow view data to be exported.
	// Once no longer needed, you can unsubscribe from the view.
	if err := view.Register(
		&view.View{
			Name:        "video_count",
			Description: "number of videos processed over time",
			Measure:     videoCount,
			Aggregation: view.Count(),
		},
		&view.View{
			Name:        "video_size",
			Description: "processed video size over time",
			Measure:     videoSize,
			Aggregation: view.Distribution(0, 5, 7, 8, 10),
			TagKeys:     []tag.Key{keyMethod, keyStatus},
		},
	); err != nil {
		log.Fatalf("Cannot register the view: %v", err)
	}

	// Record some data points...
	go func() {
		for {
			stats.Record(ctx, videoCount.M(1), videoSize.M(rand.Int63n(10)))
			// stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(keyMethod, "GET"), tag.Upsert(keyStatus, "200")}, videoCount.M(1), videoSize.M(rand.Int63n(10)))
			<-time.After(time.Millisecond * time.Duration(1000+rand.Intn(400)))
		}
	}()
}

func testTimer(ctx context.Context) {
	tagKey := tag.MustNewKey("method")

	timer := metrics.NewTimerMs("test_timer", "test timer", tagKey)

	go func() {
		for {
			ctx, _ := tag.New(ctx, tag.Insert(tagKey, "test"))
			w := timer.Start()
			<-time.After(time.Millisecond * time.Duration(1000+rand.Intn(400)))
			w.Stop(ctx)
		}
	}()

}
