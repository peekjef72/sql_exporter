// package db2_exporter
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"
)

const (
	CollectorStatusError int = iota
	CollectorStatusOk
	CollectorStatusInvalidLogin
	CollectorStatusTimeout
)

// Collector is a self-contained group of SQL queries and metric families to collect from a specific database. It is
// conceptually similar to a prometheus.Collector.
type Collector interface {
	// Collect is the equivalent of prometheus.Collector.Collect() but takes a context to run in and a database to run on.
	Collect(context.Context, *sql.DB, map[string]interface{}, chan<- Metric)
	Name() (name string)
	Status() int
	SetStatus(status int)
}

// collector implements Collector. It wraps a collection of queries, metrics and the database to collect them from.
type collector struct {
	config     *CollectorConfig
	queries    []*Query
	logContext []interface{}
	logger     *slog.Logger
	status     int
}

// NewCollector returns a new Collector with the given configuration and database. The metrics it creates will all have
// the provided const labels applied.
func NewCollector(
	logContext []interface{},
	logger *slog.Logger,
	cc *CollectorConfig,
	constLabels []*dto.LabelPair) (Collector, error) {

	logContext = append(logContext, "collector", cc.Name)

	// Maps each query to the list of metric families it populates.
	queryMFs := make(map[*QueryConfig][]*MetricFamily, len(cc.Metrics))

	// Instantiate metric families.
	for _, mc := range cc.Metrics {
		mf, err := NewMetricFamily(logContext, mc, constLabels)
		if err != nil {
			return nil, err
		}
		mfs, found := queryMFs[mc.Query()]
		if !found {
			mfs = make([]*MetricFamily, 0, 2)
		}
		queryMFs[mc.Query()] = append(mfs, mf)
	}

	// Instantiate queries.
	queries := make([]*Query, 0, len(cc.Metrics))
	for qc, mfs := range queryMFs {
		q, err := NewQuery(logContext, logger, qc, mfs...)
		if err != nil {
			return nil, err
		}
		queries = append(queries, q)
	}

	c := collector{
		config:     cc,
		queries:    queries,
		logContext: logContext,
		logger:     logger,
	}
	if c.config.MinInterval > 0 {
		var logCtx []any

		logCtx = append(logCtx, logContext...)
		logCtx = append(logCtx, "msg", fmt.Sprintf("Non-zero min_interval (%s), using cached collector.", c.config.MinInterval))
		logger.Debug("stack", logCtx...)
		return newCachingCollector(&c), nil
	}
	return &c, nil
}

// GetName implement GetName for collector
// obtain collector name for collector_status metric
func (c *collector) Name() string {
	return c.config.Name
}

// GetStatus implement GetStatus for collector
// obtain the status of collector scripts execution
func (c *collector) Status() int {
	return c.status
}

// SetStatus implement SetStatus for collector
// set the status error of collector scripts execution
func (c *collector) SetStatus(status int) {
	c.status = status
}

// Collect implements Collector.
func (c *collector) Collect(
	ctx context.Context,
	conn *sql.DB,
	symbols_table map[string]interface{},
	ch chan<- Metric) {
	var (
		wg sync.WaitGroup
	)
	wg.Add(len(c.queries))
	c.status = CollectorStatusError
	status := CollectorStatusOk
	for _, q := range c.queries {
		go func(q *Query) {
			defer wg.Done()
			q.Collect(ctx, conn, symbols_table, ch)
		}(q)
	}
	// Only return once all queries have been processed
	wg.Wait()
	c.logger.Debug(fmt.Sprintf("check collector status for %s", c.Name()))
	// logContext always contains at least 2 pairs of values: (target, target_name) (collector, collector_name)
	// so if length if greater than 4 there were errors !
	if len(c.logContext) > 4 {
		status = CollectorStatusError
	}
	// set collector execution status
	c.status = status
}

// newCachingCollector returns a new Collector wrapping the provided raw Collector.
func newCachingCollector(rawColl *collector) Collector {
	cc := &cachingCollector{
		rawColl:     rawColl,
		minInterval: time.Duration(rawColl.config.MinInterval),
		cacheSem:    make(chan time.Time, 1),
	}
	cc.cacheSem <- time.Time{}
	return cc
}

// Collector with a cache for collected metrics. Only used when min_interval is non-zero.
type cachingCollector struct {
	// Underlying collector, which is being cached.
	rawColl *collector
	// Convenience copy of rawColl.config.MinInterval.
	minInterval time.Duration

	// Used as a non=blocking semaphore protecting the cache. The value in the channel is the time of the cached metrics.
	cacheSem chan time.Time
	// Metrics saved from the last Collect() call.
	cache []Metric
}

// GetName implement GetName for cachingCollector
// obtain collector name for collector_status metric
func (cc *cachingCollector) Name() (id string) {
	return cc.rawColl.config.Name
}

// GetStatus implement GetStatus for cachingCollector
// obtain the status of collector scripts execution
func (cc *cachingCollector) Status() int {
	return cc.rawColl.status
}

// SetStatus implement SetStatus for cachingCollector
// Set the status of collector scripts execution
func (cc *cachingCollector) SetStatus(status int) {
	cc.rawColl.status = status
}

// Collect implements Collector.
func (cc *cachingCollector) Collect(
	ctx context.Context,
	conn *sql.DB,
	symbols_table map[string]interface{},
	ch chan<- Metric) {
	if ctx.Err() != nil {
		ch <- NewInvalidMetric(cc.rawColl.logContext, ctx.Err())
		return
	}

	collTime := time.Now()
	select {
	case cacheTime := <-cc.cacheSem:
		// Have the lock.
		if age := collTime.Sub(cacheTime); age > cc.minInterval {
			// Cache contents are older than minInterval, collect fresh metrics, cache them and pipe them through.
			var logCtx []interface{}

			logCtx = append(logCtx, cc.rawColl.logContext...)
			logCtx = append(logCtx, "msg", fmt.Sprintf("Collecting fresh metrics: min_interval=%.3fs cache_age=%.3fs",
				cc.minInterval.Seconds(), age.Seconds()))
			cc.rawColl.logger.Debug("stacked", logCtx...)
			cacheChan := make(chan Metric, capMetricChan)
			cc.cache = make([]Metric, 0, len(cc.cache))
			go func() {
				cc.rawColl.Collect(ctx, conn, symbols_table, cacheChan)
				close(cacheChan)
			}()
			for metric := range cacheChan {
				cc.cache = append(cc.cache, metric)
				ch <- metric
			}
			cacheTime = collTime
		} else {
			var logCtx []interface{}

			logCtx = append(logCtx, cc.rawColl.logContext...)
			logCtx = append(logCtx, "msg", fmt.Sprintf("Returning cached metrics: min_interval=%.3fs cache_age=%.3fs",
				cc.minInterval.Seconds(), age.Seconds()))
			cc.rawColl.logger.Debug("stacked", logCtx...)
			for _, metric := range cc.cache {
				ch <- metric
			}
		}
		// Always replace the value in the semaphore channel.
		cc.cacheSem <- cacheTime

	case <-ctx.Done():
		// Context closed, record an error and return
		// TODO: increment an error counter
		ch <- NewInvalidMetric(cc.rawColl.logContext, ctx.Err())
	}
}
