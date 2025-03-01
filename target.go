package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/imdario/mergo"
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
	collectorStatusHelp = "collector scripts status 0: error - 1: ok - 2: Invalid login 3: Timeout"
)

// Target collects SQL metrics from a single sql.DB instance. It aggregates one or more Collectors and it looks much
// like a prometheus.Collector, except its Collect() method takes a Context to run in.
type Target interface {
	// Collect is the equivalent of prometheus.Collector.Collect(), but takes a context to run in.
	Collect(ctx context.Context, ch chan<- Metric, health_only bool)
	Name() string
	Config() *TargetConfig
	SetSymbol(string, any) error
	DeleteSymbol(key string)
	GetSymbolTable() map[string]any
	GetSpecificCollector() []Collector
	SetSpecificCollectorConfig(coll map[string]*CollectorConfig) error
	SetLogger(*slog.Logger)
	Lock()
	Unlock()
	CloseCnx()
}

// target implements Target. It wraps a sql.DB, which is initially nil but never changes once instantianted.
type target struct {
	// name                string
	config *TargetConfig
	// dsn                 string
	collectors []Collector
	// constLabels         prometheus.Labels
	globalConfig        *GlobalConfig
	upDesc              MetricDesc
	scrapeDurationDesc  MetricDesc
	collectorStatusDesc MetricDesc
	logContext          []interface{}

	conn   *sql.DB
	logger *slog.Logger

	symbols_table map[string]interface{}
	// to store the final dsn instead of recompe it each time
	private_dsn string

	// user specific collector reference
	collector_config    map[string]*CollectorConfig
	specific_collectors []Collector

	// to protect the data during exchange
	content_mutex *sync.Mutex
}

func build_ConstantLabels(labels map[string]string) []*dto.LabelPair {
	constLabelPairs := make([]*dto.LabelPair, 0, len(labels))
	for n, v := range labels {
		constLabelPairs = append(constLabelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(v),
		})
	}
	sort.Sort(labelPairSorter(constLabelPairs))
	return constLabelPairs
}

// NewTarget returns a new Target with the given instance name, data source name, collectors and constant labels.
// An empty target name means the exporter is running in single target mode: no synthetic metrics will be exported.
func NewTarget(
	logContext []interface{},
	tpar *TargetConfig,
	ccs []*CollectorConfig,
	constLabels prometheus.Labels,
	gc *GlobalConfig,
	logger *slog.Logger) (Target, error) {

	if tpar.Name != "" {
		logContext = append(logContext, "target", tpar.Name)
	}

	constLabelPairs := build_ConstantLabels(tpar.Labels)
	collectors := make([]Collector, 0, len(ccs))
	for _, cc := range ccs {
		c, err := NewCollector(logContext, logger, cc, constLabelPairs)
		if err != nil {
			return nil, err
		}
		collectors = append(collectors, c)
	}

	upDesc := NewAutomaticMetricDesc(
		logContext,
		gc.NameSpace+"_up",
		gc.UpMetricHelp,
		prometheus.GaugeValue,
		constLabelPairs,
	)

	scrapeDurationDesc := NewAutomaticMetricDesc(logContext,
		gc.NameSpace+"_"+scrapeDurationName,
		gc.ScrapeDurationHelp,
		prometheus.GaugeValue,
		constLabelPairs,
	)

	labels := make([]string, 1)
	labels[0] = "collector"
	collectorStatusDesc := NewAutomaticMetricDesc(logContext,
		gc.NameSpace+"_"+collectorStatusName,
		gc.CollectorStatusHelp,
		prometheus.GaugeValue, constLabelPairs,
		labels...,
	)

	symbols_table := make(map[string]interface{}, 2)

	t := target{
		config: tpar,
		// name:                tpar.Name,
		// dsn:                 string(tpar.DSN),
		collectors:          collectors,
		globalConfig:        gc,
		upDesc:              upDesc,
		scrapeDurationDesc:  scrapeDurationDesc,
		collectorStatusDesc: collectorStatusDesc,
		logContext:          logContext,
		logger:              logger,
		symbols_table:       symbols_table,
		content_mutex:       &sync.Mutex{},
	}
	return &t, nil
}

// Name implement Target.Name
// to obtain target name from interface
func (t *target) Name() string {
	return t.config.Name
}

// Config implement Target.Name for target
// to obtain target name from interface
func (t *target) Config() *TargetConfig {
	return t.config
}

// SetSymbol implement Target.SetSymbol
//
// add or update element in symbol table
//
// May be unitary key (.attribute) or sequence (.attr1.attr2.[...])
func (t *target) SetSymbol(key string, value any) error {
	symtab := t.symbols_table
	if r_val, ok := symtab[key]; ok {
		vDst := reflect.ValueOf(r_val)
		if vDst.Kind() == reflect.Map {
			if m_val, ok := r_val.(map[string]any); ok {
				opts := mergo.WithOverride
				if err := mergo.Merge(&m_val, value, opts); err != nil {
					return err
				}
			}
		} else if vDst.Kind() == reflect.Slice {
			if s_val, ok := r_val.([]any); ok {
				opts := mergo.WithOverride
				if err := mergo.Merge(&s_val, value, opts); err != nil {
					return err
				}
			}
		} else {
			symtab[key] = value
		}
	} else {
		symtab[key] = value
	}
	return nil
}

func (t *target) GetSymbolTable() map[string]any {
	return t.symbols_table
}

func (t *target) DeleteSymbol(key string) {
	delete(t.symbols_table, key)
}

func (t *target) GetSpecificCollector() []Collector {
	if t.collector_config != nil {
		return t.specific_collectors
	}
	return nil
}

func (t *target) SetSpecificCollectorConfig(colls map[string]*CollectorConfig) error {
	t.collector_config = colls

	if len(colls) > 0 {
		coll_list := make([]Collector, 0, len(colls))
		var (
			constLabelPairs []*dto.LabelPair
			err             error
		)
		t.content_mutex.Lock()
		logger := t.logger
		t.content_mutex.Unlock()

		for coll_name, coll_config := range colls {
			// there was no previous specific collectors... build list
			if t.specific_collectors == nil {
				constLabelPairs = build_ConstantLabels(t.config.Labels)
				coll, err := NewCollector(t.logContext, logger, coll_config, constLabelPairs)
				if err != nil {
					return err
				}
				coll_list = append(coll_list, coll)
				// else has previous, need to check if name found in list
			} else {
				for _, coll := range t.specific_collectors {
					if coll.Name() != coll_name {
						if constLabelPairs == nil {
							constLabelPairs = build_ConstantLabels(t.config.Labels)
						}
						coll, err = NewCollector(t.logContext, logger, coll_config, constLabelPairs)
						if err != nil {
							return err
						}
					}
					coll_list = append(coll_list, coll)
				}
			}
		}
		if len(coll_list) > 0 {
			t.specific_collectors = coll_list
		} else {
			t.specific_collectors = nil
		}
	}
	return nil

}

func (t *target) SetLogger(logger *slog.Logger) {
	t.content_mutex.Lock()
	t.logger = logger
	t.content_mutex.Unlock()
}

func (t *target) Lock() {
	t.content_mutex.Lock()
}

func (t *target) Unlock() {
	t.content_mutex.Unlock()
}

func (t *target) CloseCnx() {
	t.content_mutex.Lock()
	if t.conn != nil {
		t.conn.Close()
		t.conn = nil
	}
	t.content_mutex.Unlock()

}

// Collect implements Target.
func (t *target) Collect(ctx context.Context, ch chan<- Metric, health_only bool) {
	var (
		scrapeStart = time.Now()
		targetUp    = true
	)

	err := t.ping(ctx)
	if err != nil {
		ch <- NewInvalidMetric(t.logContext, err)
		targetUp = false
	}
	if t.config.Name != "" {
		// Export the target's `up` metric as early as we know what it should be.
		ch <- NewMetric(t.upDesc, boolToFloat64(targetUp), nil)
	}
	if health_only {
		return
	}

	var wg sync.WaitGroup
	var colls []Collector

	// Don't bother with the collectors if target is down.
	if targetUp {
		if colls = t.GetSpecificCollector(); colls != nil {
			t.SetSpecificCollectorConfig(nil)
		} else {
			colls = t.collectors
		}

		wg.Add(len(colls))
		for _, c := range colls {
			// If using a single DB connection, collectors will likely run sequentially anyway. But we might have more.
			go func(collector Collector) {
				defer wg.Done()
				collector.Collect(ctx, t.conn, t.symbols_table, ch)
			}(c)
		}
	}
	// Wait for all collectors (if any) to complete.
	wg.Wait()
	t.content_mutex.Lock()
	logger := t.logger
	t.content_mutex.Unlock()
	logger.Debug("collectors have stopped")

	if t.config.Name != "" {
		// And exporter a `collector execution status` metric for each collector once we're done scraping.
		if targetUp {
			labels_name := make([]string, 1)
			labels_name[0] = "collectorname"
			labels_value := make([]string, 1)
			for _, c := range colls {
				labels_value[0] = c.Name()
				logger.Debug(
					fmt.Sprintf("target collector['%s'] has status=%d", labels_value[0], c.Status()),
					"target", t.config.Name)
				ch <- NewMetric(t.collectorStatusDesc, float64(c.Status()), labels_value)
			}
		}
		// And export a `scrape duration` metric once we're done scraping.
		ch <- NewMetric(t.scrapeDurationDesc, float64(time.Since(scrapeStart))*1e-9, nil)
	}
}

func (t *target) hasChangedAuthKey() bool {
	var auth_key, old_auth_key, need_auth_key string
	res := false
	// if set else do nothing
	params := GetMapValueMap(t.symbols_table, "params")
	if params != nil {
		need_auth_key = GetMapValueString(params, "__need_auth_key")
		if need_auth_key == "false" {
			return res
		}
		old_auth_key = GetMapValueString(params, "__auth_key")
	} else {
		// probably first call to ping...
		res = true
	}
	auth_key = GetMapValueString(t.symbols_table, "__auth_key")
	// it has changed, so reset the dsn value so it can be recomputed
	if auth_key != old_auth_key {
		res = true
	}
	return res
}

func (t *target) ping(ctx context.Context) error {
	// Create the DB handle, if necessary. It won't usually open an actual connection, so we'll need to ping afterwards.
	// We cannot do this only once at creation time because the sql.Open() documentation says it "may" open an actual
	// connection, so it "may" actually fail to open a handle to a DB that's initially down.

	// internal dsn has already been computed
	// check if auth_key is set and has changed
	if t.hasChangedAuthKey() {
		// it has changed reset the dsn value so it can be recomputed
		if t.private_dsn != "" {
			t.private_dsn = ""
		}
		if t.conn != nil {
			t.conn.Close()
			t.conn = nil
		}
	}

	if t.conn == nil {
		if t.private_dsn == "" {
			if val, err := BuildConnection(t.logger,
				string(t.config.DSN),
				t.config.AuthConfig,
				t.symbols_table,
				false,
			); err == nil {
				t.private_dsn = val
			} else {
				return ErrorWrap(t.logContext, err)
			}
		}

		conn, err := OpenConnection(ctx,
			t.logContext,
			t.logger,
			driver_name,
			t.private_dsn,
			t.globalConfig.MaxConns, t.globalConfig.MaxIdleConns,
		)
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
			if check_login_error(err) {
				t.conn.Close()
				t.conn = nil
			}
			return ErrorWrap(t.logContext, err)
		}
	}

	if ctx.Err() != nil {
		t.conn.Close()
		t.conn = nil
		return ErrorWrap(t.logContext, ctx.Err())
	}
	return nil
}

// boolToFloat64 converts a boolean flag to a float64 value (0.0 or 1.0).
func boolToFloat64(value bool) float64 {
	if value {
		return 1.0
	}
	return 0.0
}
