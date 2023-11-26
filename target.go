package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

const (
	// Capacity for the channel to collect metrics.
	capMetricChan = 1000

	upMetricHelp        = "if the target is reachable 1, else 0 if the scrape failed"
	scrapeDurationName  = "scrape_duration_seconds"
	scrapeDurationHelp  = "How long it took to scrape the target in seconds"
	collectorStatusName = "collector_status"
	collectorStatusHelp = "collector scripts status 0: error - 1: ok"
)

// Target collects SQL metrics from a single sql.DB instance. It aggregates one or more Collectors and it looks much
// like a prometheus.Collector, except its Collect() method takes a Context to run in.
type Target interface {
	// Collect is the equivalent of prometheus.Collector.Collect(), but takes a context to run in.
	Collect(ctx context.Context, ch chan<- Metric)
	Config() *TargetConfig
	GetDeadline() time.Time
	Name() string
	SetDeadline(time.Time)
	SetLogger(log.Logger)
	SetSymbol(string, any) error
}

// target implements Target. It wraps a sql.DB, which is initially nil but never changes once instantianted.
type target struct {
	name string
	// dsn                 string
	config              *TargetConfig
	collectors          []Collector
	constLabels         prometheus.Labels
	globalConfig        *GlobalConfig
	upDesc              MetricDesc
	scrapeDurationDesc  MetricDesc
	collectorStatusDesc MetricDesc
	logContext          []interface{}

	conn     *sql.DB
	logger   log.Logger
	deadline time.Time

	// to protect the data during exchange
	content_mutex *sync.Mutex

	symtab map[string]interface{}
}

// NewTarget returns a new Target with the given instance name, data source name, collectors and constant labels.
// An empty target name means the exporter is running in single target mode: no synthetic metrics will be exported.
func NewTarget(
	logContext []interface{},
	tpar *TargetConfig,
	ccs []*CollectorConfig,
	constLabels prometheus.Labels,
	gc *GlobalConfig,
	logger log.Logger) (Target, error) {

	if tpar.Name != "" {
		logContext = append(logContext, "target", tpar.Name)
	}

	constLabelPairs := make([]*dto.LabelPair, 0, len(constLabels))
	for n, v := range constLabels {
		constLabelPairs = append(constLabelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(v),
		})
	}
	sort.Sort(labelPairSorter(constLabelPairs))

	collectors := make([]Collector, 0, len(ccs))
	for _, cc := range ccs {
		c, err := NewCollector(logContext, logger, cc, constLabelPairs)
		if err != nil {
			return nil, err
		}
		collectors = append(collectors, c)
	}
	upMetricName := gc.NameSpace + "_up"
	upDesc := NewAutomaticMetricDesc(logContext,
		upMetricName,
		upMetricHelp,
		prometheus.GaugeValue,
		constLabelPairs)
	scrapeDurationDesc := NewAutomaticMetricDesc(logContext,
		gc.NameSpace+"_"+scrapeDurationName,
		scrapeDurationHelp,
		prometheus.GaugeValue,
		constLabelPairs)

	collectorStatusDesc := NewAutomaticMetricDesc(logContext,
		gc.NameSpace+"_"+collectorStatusName,
		collectorStatusHelp,
		prometheus.GaugeValue, constLabelPairs,
		"collectorname")

	symbols_table := make(map[string]interface{}, 2)

	t := target{
		name:                tpar.Name,
		config:              tpar,
		collectors:          collectors,
		constLabels:         constLabels,
		globalConfig:        gc,
		upDesc:              upDesc,
		scrapeDurationDesc:  scrapeDurationDesc,
		collectorStatusDesc: collectorStatusDesc,
		logContext:          logContext,
		logger:              logger,
		symtab:              symbols_table,
	}
	return &t, nil
}

// Collect implements Target.
func (t *target) Collect(ctx context.Context, ch chan<- Metric) {
	var (
		scrapeStart = time.Now()
		targetUp    = true
	)

	err := t.ping(ctx)
	if err != nil {
		ch <- NewInvalidMetric(t.logContext, err)
		targetUp = false
	}
	if t.name != "" {
		// Export the target's `up` metric as early as we know what it should be.
		ch <- NewMetric(t.upDesc, boolToFloat64(targetUp), nil, nil)
	}

	var wg sync.WaitGroup
	// Don't bother with the collectors if target is down.
	if targetUp {
		wg.Add(len(t.collectors))
		t.content_mutex.Lock()
		logger := t.logger
		t.content_mutex.Unlock()
		level.Debug(logger).Log(
			"collid", t.name,
			"msg", fmt.Sprintf("target: send %d collector(s)", len(t.collectors)))
		for _, c := range t.collectors {
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			level.Debug(logger).Log(
				"target", t.name,
				"collid", c.GetId(),
				"msg", "start collecting")
			// If using a single DB connection, collectors will likely run sequentially anyway. But we might have more.
			go func(collector Collector) {
				defer wg.Done()
				collector.Collect(ctx, t.conn, t.symtab, ch)
			}(c)
		}
	}
	// Wait for all collectors (if any) to complete.
	wg.Wait()

	t.content_mutex.Lock()
	logger := t.logger
	t.content_mutex.Unlock()
	level.Debug(logger).Log("msg", "collectors have stopped")

	if t.name != "" {
		// And exporter a `collector execution status` metric for each collector once we're done scraping.
		if targetUp {
			t.content_mutex.Lock()
			logger := t.logger
			t.content_mutex.Unlock()
			labels_name := make([]string, 1)
			labels_name[0] = "collectorname"
			labels_value := make([]string, 1)
			for _, c := range t.collectors {
				labels_value[0] = c.GetName()
				level.Debug(logger).Log(
					"collid", t.name,
					"msg", fmt.Sprintf("target collector['%s'] collid=[%s] has status [%d]", labels_value[0], c.GetId(), c.GetStatus()))
				ch <- NewMetric(t.collectorStatusDesc, float64(c.GetStatus()), labels_name, labels_value)
			}
		}
		// And export a `scrape duration` metric once we're done scraping.
		ch <- NewMetric(t.scrapeDurationDesc, float64(time.Since(scrapeStart))*1e-9, nil, nil)
	}
}

func (t *target) ping(ctx context.Context) error {
	// Create the DB handle, if necessary. It won't usually open an actual connection, so we'll need to ping afterwards.
	// We cannot do this only once at creation time because the sql.Open() documentation says it "may" open an actual
	// connection, so it "may" actually fail to open a handle to a DB that's initially down.
	if t.conn == nil {
		conn, err := OpenConnection(ctx, t.logContext, t.logger, string(t.config.DSN),
			t.globalConfig.MaxConns, t.globalConfig.MaxIdleConns, t.symtab)
		if err != nil {
			if err != ctx.Err() {
				return ErrorWrap(t.logContext, err)
			}
			// if err == ctx.Err() fall through
		} else {
			t.conn = conn
		}
	}

	// If we have a handle and the context is not closed, test whether the database is up.
	if t.conn != nil && ctx.Err() == nil {
		var err error
		// Ping up to max_connections + 1 times as long as the returned error is driver.ErrBadConn, to purge the connection
		// pool of bad connections. This might happen if the previous scrape timed out and in-flight queries got canceled.
		for i := 0; i <= t.globalConfig.MaxConns; i++ {
			if err = PingDB(ctx, t.conn); err != driver.ErrBadConn {
				break
			}
		}
		if err != nil {
			return ErrorWrap(t.logContext, err)
		}
	}

	if ctx.Err() != nil {
		return ErrorWrap(t.logContext, ctx.Err())
	}
	return nil
}

// to obtain target name from interface
func (t *target) Name() string {
	return t.name
}

// Config implement Target.Name for target
// to obtain target name from interface
func (t *target) Config() *TargetConfig {
	return t.config
}

// SetSymbol implement Target.SetSymbol
func (t *target) SetSymbol(key string, value any) error {

	t.symtab[key] = value
	return nil
}

// Getter for deadline
func (t *target) GetDeadline() time.Time {
	return t.deadline
}

// Setter for deadline
func (t *target) SetDeadline(tt time.Time) {
	t.deadline = tt
}

func (t *target) SetLogger(logger log.Logger) {
	t.content_mutex.Lock()
	t.logger = logger
	t.content_mutex.Unlock()
}

// boolToFloat64 converts a boolean flag to a float64 value (0.0 or 1.0).
func boolToFloat64(value bool) float64 {
	if value {
		return 1.0
	}
	return 0.0
}
