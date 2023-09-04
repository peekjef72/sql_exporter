package main

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var dsnOverride = kingpin.Flag("config.data-source-name", "Data source name to override the value in the configuration file with.").String()

// Exporter is a prometheus.Gatherer that gathers SQL metrics from targets and merges them with the default registry.
type Exporter interface {
	prometheus.Gatherer

	// WithContext returns a (single use) copy of the Exporter, which will use the provided context for Gather() calls.
	WithContext(context.Context, Target) Exporter
	// Config returns the Exporter's underlying Config object.
	Config() *Config
	Logger() log.Logger
	FindTarget(string) (Target, error)
	GetFirstTarget() (Target, error)
}

type exporter struct {
	config  *Config
	targets []Target

	cur_target Target
	ctx        context.Context
	logger     log.Logger
}

// NewExporter returns a new Exporter with the provided config.
func NewExporter(configFile string, logger log.Logger) (Exporter, error) {
	c, err := Load(configFile, logger)
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
		target, err := NewTarget(logContext, t.Name, string(t.DSN),
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
		config:  c,
		targets: targets,
		ctx:     context.Background(),
		logger:  logger,
	}, nil
}

func (e *exporter) WithContext(ctx context.Context, t Target) Exporter {
	return &exporter{
		config:     e.config,
		targets:    e.targets,
		cur_target: t,
		ctx:        ctx,
		logger:     e.logger,
	}
}

// Gather implements prometheus.Gatherer.
func (e *exporter) Gather() ([]*dto.MetricFamily, error) {
	var (
		metricChan = make(chan Metric, capMetricChan)
		errs       prometheus.MultiError
	)

	var wg sync.WaitGroup
	// wg.Add(len(e.targets))
	// for _, t := range e.targets {
	// 	go func(target Target) {
	// 		defer wg.Done()
	// 		target.Collect(e.ctx, metricChan)
	// 	}(t)
	// }
	// add only cur target
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

// Logger implements Exporter.
func (e *exporter) Logger() log.Logger {
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
		return t_found, fmt.Errorf("target '%s' not found", tname)
	}
	return t_found, nil
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
