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

// Set sets the value to `v`.
func (i *Int64) Set(ctx context.Context, v int64) {
	i.value = v
	i.record(ctx)
}

// Inc increments the inner value by value `v`.
func (i *Int64) Inc(ctx context.Context, v int64) {
	i.value += v
	i.record(ctx)
}

// Set sets the value of the gauge to value `v`.
func (i *Int64) record(ctx context.Context) {
	stats.Record(ctx, i.measureCt.M(i.value))
}

// NewInt64 creates a new Int64 Gauge
func NewInt64(name, desc string, unit string, keys ...tag.Key) *Int64 {
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

// NewInt64Gauge is just the alias of NewInt64
func NewInt64Gauge(name, desc string, unit string, keys ...tag.Key) *Int64 {
	return NewInt64(name, desc, unit, keys...)
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

// NewInt64Counter creates a new Int64 with counter vie
// if what you want is just a counter please use NewCounter instead
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

// NewInt64WithSummarizer creates a new Int64 with Summarizer
func NewInt64WithSummarizer(name, desc string, unit string, keys ...tag.Key) *Int64 {
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

type Counter Int64

// Tick triggers a record with value 1
func (c *Counter) Tick(ctx context.Context) {
	(*Int64)(c).Set(ctx, 1)
}

func NewCounter(name, desc string, keys ...tag.Key) *Counter {
	return (*Counter)(NewInt64WithCounter(name, desc, "", keys...))
}

var tagCategory = tag.MustNewKey("category")

type Int64WithCategory struct {
	value map[string]int64

	measureCt *stats.Int64Measure
	view      *view.View
}

// Set sets the value to `v`.
func (i *Int64WithCategory) Set(ctx context.Context, category string, v int64) {
	ctx, _ = tag.New(ctx, tag.Insert(tagCategory, category))

	if _, ok := i.value[category]; !ok {
		i.value[category] = 0
	}
	i.value[category] = v
	stats.Record(ctx, i.measureCt.M(i.value[category]))
}

// Inc increments the inner value by value `v`.
func (i *Int64WithCategory) Inc(ctx context.Context, category string, v int64) {
	ctx, _ = tag.New(ctx, tag.Insert(tagCategory, category))

	if _, ok := i.value[category]; !ok {
		i.value[category] = 0
	}
	i.value[category] += v
	stats.Record(ctx, i.measureCt.M(i.value[category]))
}

func NewInt64WithCategory(name, desc string, unit string, keys ...tag.Key) *Int64WithCategory {
	keys = append(keys, tagCategory)
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

	value := make(map[string]int64)

	return &Int64WithCategory{
		measureCt: iMeasure,
		view:      iView,
		value:     value,
	}
}

type CounterWithCategory Int64WithCategory

// Tick triggers a record with value 1
func (c *CounterWithCategory) Tick(ctx context.Context, category string) {
	(*Int64WithCategory)(c).Set(ctx, category, 1)
}

func NewCounterWithCategory(name, desc string, keys ...tag.Key) *CounterWithCategory {
	return (*CounterWithCategory)(NewInt64WithCategory(name, desc, "", keys...))
}
