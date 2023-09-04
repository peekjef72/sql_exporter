package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
)

// Load attempts to parse the given config file and return a Config object.
func Load(configFile string, logger log.Logger) (*Config, error) {
	level.Info(logger).Log("msg", fmt.Sprintf("Loading configuration from %s", configFile))
	buf, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	c := Config{
		configFile: configFile,
		logger:     logger,
	}
	err = yaml.Unmarshal(buf, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

//
// Top-level config
//

// Config is a collection of targets and collectors.
type Config struct {
	Globals        *GlobalConfig      `yaml:"global"`
	CollectorFiles []string           `yaml:"collector_files,omitempty"`
	Targets        []*TargetConfig    `yaml:"targets,omitempty"`
	Collectors     []*CollectorConfig `yaml:"collectors,omitempty"`

	configFile string
	logger     log.Logger

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for Config.
func (c *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Config
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one target in `targets` must be defined")
	}
	if c.Globals.NameSpace != "" {
		exporter_namespace = c.Globals.NameSpace
	}
	// Load any externally defined collectors.
	if err := c.loadCollectorFiles(); err != nil {
		return err
	}

	// Populate collector references for the target/jobs.
	colls := make(map[string]*CollectorConfig)
	for _, coll := range c.Collectors {
		// Set the min interval to the global default if not explicitly set.
		if coll.MinInterval < 0 {
			coll.MinInterval = c.Globals.MinInterval
		}
		if _, found := colls[coll.Name]; found {
			return fmt.Errorf("duplicate collector name: %s", coll.Name)
		}
		colls[coll.Name] = coll
	}
	for _, t := range c.Targets {
		if len(t.TargetsFiles) > 0 {
			err := c.loadTargetsFiles(t.TargetsFiles)
			if err != nil {
				return err
			}
		}
	}

	for _, t := range c.Targets {
		cs, err := resolveCollectorRefs(t.CollectorRefs, colls, fmt.Sprintf("target %q", t.Name))
		if err != nil {
			return err
		}
		t.collectors = cs
	}

	// Check for empty/duplicate target names/data source names
	tnames := make(map[string]interface{})
	dsns := make(map[string]interface{})
	for _, t := range c.Targets {
		if len(t.TargetsFiles) > 0 {
			continue
		}
		if t.Name == "" {
			return fmt.Errorf("empty target name in static config %+v", t)
		}
		if _, ok := tnames[t.Name]; ok {
			return fmt.Errorf("duplicate target name %q in target %+v", t.Name, t)
		}
		tnames[t.Name] = nil

		if _, ok := dsns[string(t.DSN)]; ok {
			return fmt.Errorf("duplicate data source definition %q in target %+v", t.Name, t)
		}
		dsns[string(t.DSN)] = nil
	}

	return checkOverflow(c.XXX, "config")
}

// YAML marshals the config into YAML format.
func (c *Config) YAML() ([]byte, error) {
	return yaml.Marshal(c)
}

// loadCollectorFiles resolves all collector file globs to files and loads the collectors they define.
func (c *Config) loadCollectorFiles() error {
	baseDir := filepath.Dir(c.configFile)
	for _, cfglob := range c.CollectorFiles {
		// Resolve relative paths by joining them to the configuration file's directory.
		if len(cfglob) > 0 && !filepath.IsAbs(cfglob) {
			cfglob = filepath.Join(baseDir, cfglob)
		}

		// Resolve the glob to actual filenames.
		cfs, err := filepath.Glob(cfglob)
		if err != nil {
			// The only error can be a bad pattern.
			return fmt.Errorf("error resolving collector files for %s: %s", cfglob, err)
		}

		// And load the CollectorConfig defined in each file.
		for _, cf := range cfs {
			level.Debug(c.logger).Log("msg", fmt.Sprintf("Loading collector metrics from %s", cf))
			buf, err := os.ReadFile(cf)
			if err != nil {
				return err
			}

			cc := CollectorConfig{}
			err = yaml.Unmarshal(buf, &cc)
			if err != nil {
				return err
			}
			c.Collectors = append(c.Collectors, &cc)
			level.Info(c.logger).Log("msg", fmt.Sprintf("Loaded collector %q from %s", cc.Name, cf))
		}
	}

	return nil
}

// loadTargetsFiles resolves all targets file globs to files and loads the targets they define.
func (c *Config) loadTargetsFiles(targetFilepath []string) error {
	baseDir := filepath.Dir(c.configFile)
	for _, tfglob := range targetFilepath {
		// Resolve relative paths by joining them to the configuration file's directory.
		if len(tfglob) > 0 && !filepath.IsAbs(tfglob) {
			tfglob = filepath.Join(baseDir, tfglob)
		}

		// Resolve the glob to actual filenames.
		tfs, err := filepath.Glob(tfglob)
		if err != nil {
			// The only error can be a bad pattern.
			return fmt.Errorf("error resolving collector files for %s: %s", tfglob, err)
		}

		// And load the CollectorConfig defined in each file.
		for _, tf := range tfs {
			level.Debug(c.logger).Log("msg", fmt.Sprintf("Loading targets from %s", tf))
			buf, err := os.ReadFile(tf)
			if err != nil {
				return err
			}

			target := TargetConfig{}
			err = yaml.Unmarshal(buf, &target)
			if err != nil {
				return err
			}
			target.setFromFile(tf)
			c.Targets = append(c.Targets, &target)
			level.Debug(c.logger).Log("msg", fmt.Sprintf("Loaded target %q from %s", target.Name, tf))
		}
	}

	return nil
}

// GlobalConfig contains globally applicable defaults.
type GlobalConfig struct {
	MinInterval   model.Duration `yaml:"min_interval"`          // minimum interval between query executions, default is 0
	ScrapeTimeout model.Duration `yaml:"scrape_timeout"`        // per-scrape timeout, global
	TimeoutOffset model.Duration `yaml:"scrape_timeout_offset"` // offset to subtract from timeout in seconds
	MaxConns      int            `yaml:"max_connections"`       // maximum number of open connections to any one target
	MaxIdleConns  int            `yaml:"max_idle_connections"`  // maximum number of idle connections to any one target
	NameSpace     string         `yaml:"namespace"`             // prefix to add to all metric name (prifx + '_')

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for GlobalConfig.
func (g *GlobalConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Default to running the queries on every scrape.
	g.MinInterval = model.Duration(0)
	// Default to 10 seconds, since Prometheus has a 10 second scrape timeout default.
	g.ScrapeTimeout = model.Duration(10 * time.Second)
	// Default to .5 seconds.
	g.TimeoutOffset = model.Duration(500 * time.Millisecond)
	g.MaxConns = 3
	g.MaxIdleConns = 3

	type plain GlobalConfig
	if err := unmarshal((*plain)(g)); err != nil {
		return err
	}

	if g.NameSpace == "" {
		g.NameSpace = exporter_namespace
	}

	if g.TimeoutOffset <= 0 {
		return fmt.Errorf("global.scrape_timeout_offset must be strictly positive, have %s", g.TimeoutOffset)
	}

	return checkOverflow(g.XXX, "global")
}

//
// Targets
//

// TargetConfig defines a DSN and a set of collectors to be executed on it.
type TargetConfig struct {
	Name          string            `yaml:"name"`                    // data source name to connect to
	DSN           Secret            `yaml:"data_source_name"`        // data source definition to connect to
	Labels        map[string]string `yaml:"labels,omitempty"`        // labels to apply to all metrics collected from the targets
	CollectorRefs []string          `yaml:"collectors"`              // names of collectors to execute on the target
	TargetsFiles  []string          `yaml:"targets_files,omitempty"` // slice of path and pattern for files that contains targets

	collectors []*CollectorConfig // resolved collector references
	fromFile   string             // filepath if loaded from targets_files pattern

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// Collectors returns the collectors referenced by the target, resolved.
func (t *TargetConfig) Collectors() []*CollectorConfig {
	return t.collectors
}

// set fromFile for target when read from targets_files directive
func (t *TargetConfig) setFromFile(file_path string) {
	t.fromFile = file_path
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for TargetConfig.
func (t *TargetConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain TargetConfig
	if err := unmarshal((*plain)(t)); err != nil {
		return err
	}
	// Check required fields

	if len(t.TargetsFiles) == 0 {
		if t.Name == "" {
			return fmt.Errorf("empty target name in target %+v", t)
		}

		if t.DSN == "" {
			return fmt.Errorf("missing data_source_name for target %+v", t)
		}
		checkCollectorRefs(t.CollectorRefs, "target")

		if len(t.Labels) > 0 {
			err := t.checkLabelCollisions()
			if err != nil {
				return err
			}
		}
	} else {
		for _, file := range t.TargetsFiles {
			if file == "" {
				return fmt.Errorf("missing targets_files pattern")
			}
		}
	}
	return checkOverflow(t.XXX, "target")
}

// checkLabelCollisions checks for label collisions between StaticConfig labels and Metric labels.
func (t *TargetConfig) checkLabelCollisions() error {
	sclabels := make(map[string]interface{})
	for _, l := range t.Labels {
		sclabels[l] = nil
	}

	for _, c := range t.collectors {
		for _, m := range c.Metrics {
			for _, l := range m.KeyLabels {
				if _, ok := sclabels[l]; ok {
					return fmt.Errorf(
						"label collision in target %q: label %q is defined both by a static_config and by metric %q of collector %q",
						t.Name, l, m.Name, c.Name)
				}
			}
		}
	}
	return nil
}

//
// Collectors
//

// CollectorConfig defines a set of metrics and how they are collected.
type CollectorConfig struct {
	Name        string          `yaml:"collector_name"`         // name of this collector
	NameSpace   string          `yaml:"namespace"`              // prefix to add to all metric name (prifx + '_')
	MinInterval model.Duration  `yaml:"min_interval,omitempty"` // minimum interval between query executions
	Metrics     []*MetricConfig `yaml:"metrics"`                // metrics/queries defined by this collector
	Queries     []*QueryConfig  `yaml:"queries,omitempty"`      // named queries defined by this collector

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for CollectorConfig.
func (c *CollectorConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Default to undefined (a negative value) so it can be overridden by the global default when not explicitly set.
	c.MinInterval = -1

	type plain CollectorConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}

	if len(c.XXX) > 0 {
		return checkOverflow(c.XXX, "collector")
	}
	if len(c.Metrics) == 0 {
		return fmt.Errorf("no metrics defined for collector %q", c.Name)
	}

	if c.NameSpace == "" {
		c.NameSpace = exporter_namespace
	}

	// Set metric.query for all metrics: resolve query references (if any) and generate QueryConfigs for literal queries.
	queries := make(map[string]*QueryConfig, len(c.Queries))
	for _, query := range c.Queries {
		queries[query.Name] = query
	}
	for _, metric := range c.Metrics {
		if !strings.HasPrefix(metric.Name, c.NameSpace) {
			metric.Name = fmt.Sprintf("%s_%s", c.NameSpace, metric.Name)
		}

		if metric.QueryRef != "" {
			query, found := queries[metric.QueryRef]
			if !found {
				return fmt.Errorf("unresolved query_ref %q in metric %q of collector %q", metric.QueryRef, metric.Name, c.Name)
			}
			metric.query = query
			query.metrics = append(query.metrics, metric)
		} else {
			// For literal queries generate a QueryConfig with a name based off collector and metric name.
			metric.query = &QueryConfig{
				Name:  metric.Name,
				Query: metric.QueryLiteral,
			}
		}
	}

	return checkOverflow(c.XXX, "collector")
}

// MetricConfig defines a Prometheus metric, the SQL query to populate it and the mapping of columns to metric
// keys/values.
type MetricConfig struct {
	Name         string            `yaml:"metric_name"`             // the Prometheus metric name
	TypeString   string            `yaml:"type"`                    // the Prometheus metric type
	Help         string            `yaml:"help"`                    // the Prometheus metric help text
	KeyLabels    []string          `yaml:"key_labels,omitempty"`    // expose these columns as labels from SQL
	StaticLabels map[string]string `yaml:"static_labels,omitempty"` // fixed key/value pairs as static labels
	ValueLabel   string            `yaml:"value_label,omitempty"`   // with multiple value columns, map their names under this label
	Values       []string          `yaml:"values"`                  // expose each of these columns as a value, keyed by column name
	QueryLiteral string            `yaml:"query,omitempty"`         // a literal query
	QueryRef     string            `yaml:"query_ref,omitempty"`     // references a query in the query map

	valueType prometheus.ValueType // TypeString converted to prometheus.ValueType
	query     *QueryConfig         // QueryConfig resolved from QueryRef or generated from Query

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// ValueType returns the metric type, converted to a prometheus.ValueType.
func (m *MetricConfig) ValueType() prometheus.ValueType {
	return m.valueType
}

// Query returns the query defined (as a literal) or referenced by the metric.
func (m *MetricConfig) Query() *QueryConfig {
	return m.query
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for MetricConfig.
func (m *MetricConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain MetricConfig
	if err := unmarshal((*plain)(m)); err != nil {
		return err
	}

	// Check required fields
	if m.Name == "" {
		return fmt.Errorf("missing name for metric %+v", m)
	}
	if m.TypeString == "" {
		m.TypeString = "gauge"
		// return fmt.Errorf("missing type for metric %q", m.Name)
	}
	if m.Help == "" {
		return fmt.Errorf("missing help for metric %q", m.Name)
	}
	if (m.QueryLiteral == "") == (m.QueryRef == "") {
		return fmt.Errorf("exactly one of query and query_ref must be specified for metric %q", m.Name)
	}

	switch strings.ToLower(m.TypeString) {
	case "counter":
		m.valueType = prometheus.CounterValue
	case "gauge":
		m.valueType = prometheus.GaugeValue
	default:
		return fmt.Errorf("unsupported metric type: %s", m.TypeString)
	}

	// Check for duplicate key labels
	for i, li := range m.KeyLabels {
		checkLabel(li, "metric", m.Name)
		for _, lj := range m.KeyLabels[i+1:] {
			if li == lj {
				return fmt.Errorf("duplicate key label %q for metric %q", li, m.Name)
			}
		}
		if m.ValueLabel == li {
			return fmt.Errorf("duplicate label %q (defined in both key_labels and value_label) for metric %q", li, m.Name)
		}
	}

	if len(m.Values) == 0 {
		return fmt.Errorf("no values defined for metric %q", m.Name)
	}

	for i, value := range m.Values {
		if value != "" {
			m.Values[i] = strings.ToLower(value)
		}
	}

	if len(m.Values) > 1 {
		// Multiple value columns but no value label to identify them
		if m.ValueLabel == "" {
			return fmt.Errorf("value_label must be defined for metric with multiple values %q", m.Name)
		}
		checkLabel(m.ValueLabel, "value_label for metric", m.Name)
	}

	return checkOverflow(m.XXX, "metric")
}

// QueryConfig defines a named query, to be referenced by one or multiple metrics.
type QueryConfig struct {
	Name  string `yaml:"query_name"` // the query name, to be referenced via `query_ref`
	Query string `yaml:"query"`      // the named query

	metrics []*MetricConfig // metrics referencing this query

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline" json:"-"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for QueryConfig.
func (q *QueryConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain QueryConfig
	if err := unmarshal((*plain)(q)); err != nil {
		return err
	}

	// Check required fields
	if q.Name == "" {
		return fmt.Errorf("missing name for query %+v", *q)
	}
	if q.Query == "" {
		return fmt.Errorf("missing query literal for query %q", q.Name)
	}

	q.metrics = make([]*MetricConfig, 0, 2)

	return checkOverflow(q.XXX, "metric")
}

// Secret special type for storing secrets.
type Secret string

// UnmarshalYAML implements the yaml.Unmarshaler interface for Secrets.
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Secret
	return unmarshal((*plain)(s))
}

// MarshalYAML implements the yaml.Marshaler interface for Secrets.
func (s Secret) MarshalYAML() (interface{}, error) {
	if s != "" {
		return "<secret>", nil
	}
	return nil, nil
}
func checkCollectorRefs(collectorRefs []string, ctx string) error {
	// At least one collector, no duplicates
	if len(collectorRefs) == 0 {
		return fmt.Errorf("no collectors defined for %s", ctx)
	}
	for i, ci := range collectorRefs {
		for _, cj := range collectorRefs[i+1:] {
			if ci == cj {
				return fmt.Errorf("duplicate collector reference %q in %s", ci, ctx)
			}
		}
	}
	return nil
}

func resolveCollectorRefs(
	collectorRefs []string, collectors map[string]*CollectorConfig, ctx string) ([]*CollectorConfig, error) {
	resolved := make([]*CollectorConfig, 0, len(collectorRefs))
	for _, cref := range collectorRefs {
		c, found := collectors[cref]
		if !found {
			return nil, fmt.Errorf("unknown collector %q referenced in %s", cref, ctx)
		}
		resolved = append(resolved, c)
	}
	return resolved, nil
}

func checkLabel(label string, ctx ...string) error {
	if label == "" {
		return fmt.Errorf("empty label defined in %s", strings.Join(ctx, " "))
	}
	if label == "job" || label == "instance" {
		return fmt.Errorf("reserved label %q redefined in %s", label, strings.Join(ctx, " "))
	}
	return nil
}

func checkOverflow(m map[string]interface{}, ctx string) error {
	if len(m) > 0 {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Errorf("unknown fields in %s: %s", ctx, strings.Join(keys, ", "))
	}
	return nil
}

func ErrorWrap(logContext []interface{}, err error) error {
	var logCtx []interface{}
	var ErrMissingValue = errors.New("(MISSING)")

	if err == nil {
		return nil
	}
	logCtx = append(logCtx, logContext...)
	logCtx = append(logCtx, "errmsg", fmt.Sprintf("%q", err))

	if len(logCtx)%2 != 0 {
		logCtx = append(logCtx, ErrMissingValue)
	}
	tmp := make(map[interface{}]interface{}, len(logCtx)/2+1)
	for i := 0; i < len(logCtx); i += 2 {
		tmp[logCtx[i]] = logCtx[i+1]
	}
	return fmt.Errorf("%v", tmp)
}
