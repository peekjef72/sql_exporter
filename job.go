package sql_exporter

import (
	"fmt"

	"sql_exporter/config"
	// "sql_exporter/errors"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

// Job is a collection of targets with the same collectors applied.
type Job interface {
	Targets() []Target
}

// job implements Job. It wraps the corresponding JobConfig and a set of Targets.
type job struct {
	config     *config.JobConfig
	targets    []Target
	logContext []interface{}
}

// NewJob returns a new Job with the given configuration.
func NewJob(jc *config.JobConfig, gc *config.GlobalConfig, logger log.Logger) (Job, error) {
	var logContext []interface{}
	logContext = append(logContext, "job", jc.Name)
	j := job{
		config:     jc,
		targets:    make([]Target, 0, 10),
		logContext: logContext,
	}

	for _, sc := range jc.StaticConfigs {
		for tname, dsn := range sc.Targets {
			constLabels := prometheus.Labels{
				"job":      jc.Name,
				"instance": tname,
			}
			for name, value := range sc.Labels {
				// Shouldn't happen as there are sanity checks in config, but check nonetheless.
				if _, found := constLabels[name]; found {
					var logContext []interface{}
					logContext = append(logContext, j.logContext...)
					logContext = append(logContext, "errmsg", fmt.Sprintf("duplicate label %q", name))
					return nil, fmt.Errorf("%s", logContext...)
				}
				constLabels[name] = value
			}
			t, err := NewTarget(j.logContext, tname, string(dsn), jc.Collectors(), constLabels, gc, logger)
			if err != nil {
				return nil, err
			}
			j.targets = append(j.targets, t)
		}
	}

	return &j, nil
}

func (j *job) Targets() []Target {
	return j.targets
}
