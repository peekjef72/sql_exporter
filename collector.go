// package db2_exporter
package main

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	dto "github.com/prometheus/client_model/go"
)

// Collector is a self-contained group of SQL queries and metric families to collect from a specific database. It is
// conceptually similar to a prometheus.Collector.
type Collector interface {
	// Collect is the equivalent of prometheus.Collector.Collect() but takes a context to run in and a database to run on.
	Collect(context.Context, *sql.DB, map[string]interface{}, chan<- Metric)
	GetName() (id string)
	GetId() (id string)
	GetStatus() int
	SetLogger(log.Logger)
}

// collector implements Collector. It wraps a collection of queries, metrics and the database to collect them from.
type collector struct {
	config     *CollectorConfig
	queries    []*Query
	logContext []interface{}
	status     int

	// to protect the data during exchange
	content_mutex *sync.Mutex
	logger        log.Logger
}

// NewCollector returns a new Collector with the given configuration and database. The metrics it creates will all have
// the provided const labels applied.
func NewCollector(
	logContext []interface{},
	logger log.Logger,
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
		config:        cc,
		queries:       queries,
		logContext:    logContext,
		logger:        logger,
		content_mutex: &sync.Mutex{},
	}
	if c.config.MinInterval > 0 {
		var logCtx []interface{}

		logCtx = append(logCtx, logContext...)
		logCtx = append(logCtx, "msg", fmt.Sprintf("Non-zero min_interval (%s), using cached collector.", c.config.MinInterval))
		level.Debug(logger).Log(logCtx...)
		return newCachingCollector(&c), nil
	}
	return &c, nil
}

// Collect implements Collector.
func (c *collector) Collect(
	ctx context.Context,
	conn *sql.DB,
	symbols_table map[string]interface{},
	met_ch chan<- Metric) {

	var (
		wg     sync.WaitGroup
		status int = 0
	)
	// chan to receive order from collector if something wrong with authentication
	collectChan := make(chan int, len(c.queries))

	wg.Add(len(c.queries))

	c.status = 0
	status = 1
	for _, q := range c.queries {
		go func(q *Query) {
			defer wg.Done()
			q.Collect(ctx, conn, symbols_table, met_ch, collectChan)
		}(q)
	}
	// Only return once all queries have been processed
	wg.Wait()

	// Drain collectChan in case of premature return.
	defer func() {
		for msg := range collectChan {
			if msg == 0 {
				status = msg
			}
		}
	}()

	// set collector execution status
	c.status = status

}

// GetId implement GetId for collector
// obtain collector id for log purpose
func (c *collector) GetId() string {
	return c.config.id
}

// GetName implement GetName for collector
// obtain collector name for collector_status metric
func (c *collector) GetName() string {
	return c.config.Name
}

// GetStatus implement GetStatus for collector
// obtain the status of collector scripts execution
func (c *collector) GetStatus() int {
	return c.status
}

func (c *collector) SetLogger(logger log.Logger) {
	c.content_mutex.Lock()
	c.logger = logger
	// c.client.logger = logger
	c.content_mutex.Unlock()
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
func (cc *cachingCollector) GetName() (id string) {
	return cc.rawColl.config.Name
}

// GetId implement GetId for cachingCollector
func (cc *cachingCollector) GetId() (id string) {
	return cc.rawColl.config.id
}

// GetStatus implement GetStatus for collector
// obtain the status of collector scripts execution
func (cc *cachingCollector) GetStatus() int {
	return cc.rawColl.status
}

func (cc *cachingCollector) SetLogger(logger log.Logger) {
	cc.rawColl.SetLogger(logger)
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
			level.Debug(cc.rawColl.logger).Log(logCtx...)
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
			level.Debug(cc.rawColl.logger).Log(logCtx...)
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
