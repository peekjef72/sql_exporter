package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/promslog"
)

var dsnOverride = kingpin.Flag("config.data-source-name", "Data source name to override the value in the configuration file with.").String()

var (
	ErrTargetNotFound = fmt.Errorf("target not found")
)

// Exporter is a prometheus.Gatherer that gathers SQL metrics from targets and merges them with the default registry.
type Exporter interface {
	prometheus.Gatherer

	// WithContext returns a (single use) copy of the Exporter, which will use the provided context for Gather() calls.
	WithContext(context.Context, Target) Exporter
	// Config returns the Exporter's underlying Config object.
	Config() *Config
	Targets() []Target
	Logger() *slog.Logger
	AddTarget(*TargetConfig) (Target, error)
	FindTarget(string) (Target, error)
	GetFirstTarget() (Target, error)
	SetStartTime(time.Time)
	GetStartTime() string
	SetReloadTime(time.Time)
	GetReloadTime() string

	SetLogLevel(level string)
	GetLogLevel() string
	IncreaseLogLevel(string)

	ReloadConfig() error
}

type exporter struct {
	config  *Config
	targets []Target

	cur_target    Target
	ctx           context.Context
	logger        *slog.Logger
	start_time    string
	reload_time   string
	logLevel      string
	content_mutex *sync.Mutex
}

// NewExporter returns a new Exporter with the provided config.
func NewExporter(configFile string, logger *slog.Logger, collectorName string) (Exporter, error) {
	c, err := LoadConfig(configFile, logger, collectorName)
	if err != nil {
		return nil, err
	}

	// Override the DSN if requested (and in single target mode).
	if *dsnOverride != "" {
		if len(c.Targets) > 1 {
			return nil, fmt.Errorf("the config.data-source-name flag (value %q) only applies in single target mode", *dsnOverride)
		} else {
			c.Targets[0].DSN = Secret(*dsnOverride)
		}
	}

	var targets []Target
	var logContext []interface{}
	if len(c.Targets) > 1 {
		targets = make([]Target, 0, len(c.Targets)*3)
	}
	for _, t := range c.Targets {
		if len(t.TargetsFiles) > 0 {
			continue
		}
		target, err := NewTarget(logContext, t,
			t.Collectors(), nil, c.Globals, logger)
		if err != nil {
			return nil, err
		}
		if len(c.Targets) > 1 {
			targets = append(targets, target)
		} else {
			targets = []Target{target}
		}
	}

	return &exporter{
		config:        c,
		targets:       targets,
		ctx:           context.Background(),
		logger:        logger,
		content_mutex: &sync.Mutex{},
	}, nil
}

func (e *exporter) WithContext(ctx context.Context, t Target) Exporter {
	return &exporter{
		config:        e.config,
		targets:       e.targets,
		cur_target:    t,
		ctx:           ctx,
		logger:        e.logger,
		content_mutex: e.content_mutex,
	}
}

// Gather implements prometheus.Gatherer.
func (e *exporter) Gather() ([]*dto.MetricFamily, error) {
	var (
		metricChan = make(chan Metric, capMetricChan)
		errs       prometheus.MultiError
	)

	var wg sync.WaitGroup

	wg.Add(1)
	go func(target Target) {
		defer wg.Done()
		target.Collect(e.ctx, metricChan)
	}(e.cur_target)

	// Wait for all collectors to complete, then close the channel.
	go func() {
		wg.Wait()
		close(metricChan)
	}()

	// Drain metricChan in case of premature return.
	defer func() {
		for range metricChan {
		}
	}()

	// Gather.
	dtoMetricFamilies := make(map[string]*dto.MetricFamily, 10)
	for metric := range metricChan {
		dtoMetric := &dto.Metric{}
		if err := metric.Write(dtoMetric); err != nil {
			errs = append(errs, err)
			continue
		}
		metricDesc := metric.Desc()
		dtoMetricFamily, ok := dtoMetricFamilies[metricDesc.Name()]
		if !ok {
			dtoMetricFamily = &dto.MetricFamily{}
			dtoMetricFamily.Name = proto.String(metricDesc.Name())
			dtoMetricFamily.Help = proto.String(metricDesc.Help())
			switch {
			case dtoMetric.Gauge != nil:
				dtoMetricFamily.Type = dto.MetricType_GAUGE.Enum()
			case dtoMetric.Counter != nil:
				dtoMetricFamily.Type = dto.MetricType_COUNTER.Enum()
			default:
				errs = append(errs, fmt.Errorf("don't know how to handle metric %v", dtoMetric))
				continue
			}
			dtoMetricFamilies[metricDesc.Name()] = dtoMetricFamily
		}
		dtoMetricFamily.Metric = append(dtoMetricFamily.Metric, dtoMetric)
	}

	// No need to sort metric families, prometheus.Gatherers will do that for us when merging.
	result := make([]*dto.MetricFamily, 0, len(dtoMetricFamilies))
	for _, mf := range dtoMetricFamilies {
		result = append(result, mf)
	}
	return result, errs
}

// Config implements Exporter.
func (e *exporter) Config() *Config {
	return e.config
}

// Targets implements Exporter.
func (e *exporter) Targets() []Target {
	return e.targets
}

// Logger implements Exporter.
func (e *exporter) Logger() *slog.Logger {
	return e.logger
}

// FindTarget implements Exporter.
func (e *exporter) FindTarget(tname string) (Target, error) {
	var t_found Target
	found := false
	for _, t := range e.targets {
		if tname == t.Name() {
			t_found = t
			found = true
		}
	}
	if !found {
		return t_found, ErrTargetNotFound
	}
	return t_found, nil
}

// AddTarget implements Exporter AddTarget.
// add a new dynamically created target to config
func (e *exporter) AddTarget(tg_config *TargetConfig) (Target, error) {
	var logContext []interface{}

	target, err := NewTarget(logContext,
		tg_config, tg_config.Collectors(), nil,
		e.config.Globals, e.logger)
	if err != nil {
		return nil, err
	}
	e.targets = append(e.targets, target)

	return target, nil
}

// GetFirstTarget implements Exporter.
func (e *exporter) GetFirstTarget() (Target, error) {
	var t_found Target
	if len(e.targets) == 0 {
		return t_found, fmt.Errorf("no target found")
	} else {
		t_found = e.targets[0]
	}
	return t_found, nil
}

func (e *exporter) GetStartTime() string {
	return e.start_time
}

func (e *exporter) SetStartTime(ti time.Time) {
	e.start_time = ti.Format("2006-01-02T15:04:05.000Z07:00")
}

func (e *exporter) GetReloadTime() string {
	return e.reload_time
}

func (e *exporter) SetReloadTime(ti time.Time) {
	e.reload_time = ti.Format("2006-01-02T15:04:05.000Z07:00")
}

func (e *exporter) SetLogLevel(level string) {
	e.logLevel = level
}

func (e *exporter) GetLogLevel() string {
	return e.logLevel
}

func (e *exporter) IncreaseLogLevel(new_lvl string) {
	var log func(msg string, args ...any)
	e.content_mutex.Lock()
	defer e.content_mutex.Unlock()

	if new_lvl == "" {
		switch e.logLevel {
		case "debug":
			e.logLevel = "info"
			// Level = slog.LevelInfo
		case "info":
			e.logLevel = "warn"
			// Level = slog.LevelWarn
		case "warn":
			e.logLevel = "error"
			// Level = slog.LevelError
		case "error":
			e.logLevel = "debug"
			// Level = slog.LevelDebug
		}
	} else {
		switch new_lvl {
		case "debug":
			// Level = slog.LevelDebug
			log = e.logger.Debug
		case "info":
			// Level = slog.LevelInfo
			log = e.logger.Info
		case "warn":
			// Level = slog.LevelWarn
			log = e.logger.Warn
		case "error":
			// Level = slog.LevelError
			log = e.logger.Error
		default:
			e.logger.Error(fmt.Sprintf("invalid log.level specified %s", new_lvl))
			return
		}
		if e.logLevel == new_lvl {

			log("msg", "set log.level unchanged")
			return
		}
		e.logLevel = new_lvl
	}
	logConfig.Level.Set(e.logLevel)
	e.logger = promslog.New(&logConfig)
	for _, t := range e.targets {
		t.SetLogger(e.logger)
	}
	switch e.logLevel {
	case "debug":
		log = e.logger.Debug
	case "info":
		log = e.logger.Info
	case "warn":
		log = e.logger.Warn
	case "error":
		log = e.logger.Error
	}
	log(fmt.Sprintf("set log.level to %s", e.logLevel))
}

func (e *exporter) ReloadConfig() error {
	e.content_mutex.Lock()
	configFile := e.config.configFile
	collectorName := e.config.collectorName
	e.content_mutex.Unlock()

	c, err := LoadConfig(configFile, e.logger, collectorName)
	if err != nil {
		return err
	}

	var targets []Target
	var logContext []interface{}
	if len(c.Targets) > 1 {
		targets = make([]Target, 0, len(c.Targets)*3)
	}
	for _, t := range c.Targets {
		if len(t.TargetsFiles) > 0 {
			continue
		}
		target, err := NewTarget(logContext, t, t.Collectors(), nil, c.Globals, e.logger)
		if err != nil {
			return err
		}
		if len(c.Targets) > 1 {
			targets = append(targets, target)
		} else {
			targets = []Target{target}
		}
	}

	e.content_mutex.Lock()
	e.config = c
	e.targets = targets
	e.SetReloadTime(time.Now())
	e.content_mutex.Unlock()

	return nil
}
