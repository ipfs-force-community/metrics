package metrics

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// Int64 wraps an opencensus int64 measure that is uses as a gauge.
type Int64 struct {
	value     int64
	measureCt *stats.Int64Measure
	view      *view.View
}

// NewInt64Gauge creates a new Int64Gauge
func NewInt64Gauge(name, desc string, unit string, keys ...tag.Key) *Int64 {
	if unit == "" {
		unit = stats.UnitDimensionless
	}

	iMeasure := stats.Int64(name, desc, unit)

	iView := &view.View{
		Name:        name,
		Measure:     iMeasure,
		Description: desc,
		Aggregation: view.LastValue(),
		TagKeys:     keys,
	}
	if err := view.Register(iView); err != nil {
		// a panic here indicates a developer error when creating a view.
		// Since this method is called in init() methods, this panic when hit
		// will cause running the program to fail immediately.
		panic(err)
	}

	return &Int64{
		measureCt: iMeasure,
		view:      iView,
	}
}

// NewInt64Gauge creates a new Int64 with buckets.
func NewInt64WithBuckets(name, desc string, unit string, bounds []float64, tagKeys ...tag.Key) *Int64 {
	if unit == "" {
		unit = stats.UnitDimensionless
	}

	iMeasure := stats.Int64(name, desc, unit)

	iView := &view.View{
		Name:        name,
		Measure:     iMeasure,
		Description: desc,
		Aggregation: view.Distribution(bounds...),
		TagKeys:     tagKeys,
	}
	if err := view.Register(iView); err != nil {
		// a panic here indicates a developer error when creating a view.
		// Since this method is called in init() methods, this panic when hit
		// will cause running the program to fail immediately.
		panic(err)
	}

	return &Int64{
		measureCt: iMeasure,
		view:      iView,
	}
}

// NewInt64Counter creates a new Int64 with counter
func NewInt64WithCounter(name, desc string, unit string, keys ...tag.Key) *Int64 {
	if unit == "" {
		unit = stats.UnitDimensionless
	}

	iMeasure := stats.Int64(name, desc, unit)
	iView := &view.View{
		Name:        name,
		Measure:     iMeasure,
		Description: desc,
		TagKeys:     keys,
		Aggregation: view.Count(),
	}
	if err := view.Register(iView); err != nil {
		// a panic here indicates a developer error when creating a view.
		// Since this method is called in init() methods, this panic when hit
		// will cause running the program to fail immediately.
		panic(err)
	}

	return &Int64{
		measureCt: iMeasure,
		view:      iView,
	}
}

// Set sets the value to `v`.
func (c *Int64) Set(ctx context.Context, v int64) {
	c.value = v
	c.record(ctx)
}

// Inc increments the inner value by value `v`.
func (c *Int64) Inc(ctx context.Context, v int64) {
	c.value += v
	c.record(ctx)
}

// Set sets the value of the gauge to value `v`.
func (c *Int64) record(ctx context.Context) {
	stats.Record(ctx, c.measureCt.M(c.value))
}
