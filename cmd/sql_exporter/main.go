package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	_ "net/http/pprof"

	"github.com/peekjef72/sql_exporter"

	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

const (
	// Constant values
	metricsPublishingPort = ":9399"
	exporter_name         = "mssql_exporter"
)

var (
	listenAddress = kingpin.Flag("web.listen-address", "The address to listen on for HTTP requests.").Default(metricsPublishingPort).String()
	metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose collector's internal metrics.").Default("/metrics").String()
	configFile    = kingpin.Flag("config.file", "MSSQL Exporter configuration file.").Short('c').Default("config/config.yml").String()
	debug_flag    = kingpin.Flag("debug", "debug connection checks.").Short('d').Default("false").Bool()
	dry_run       = kingpin.Flag("dry-run", "check exporter configuration file and try to collect a target then exit.").Short('n').Default("false").Bool()
	target_name   = kingpin.Flag("target", "In dry-run mode specify the target name, else ignored.").Short('t').String()
	// alsologtostderr = kingpin.Flag("alsologtostderr", "log to standard error as well as files.").Default("true").Bool()
)

func init() {
	prometheus.MustRegister(version.NewCollector(exporter_name))
}

func main() {

	logConfig := promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, &logConfig)
	kingpin.Version(version.Print(exporter_name)).VersionFlag.Short('V')
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	if os.Getenv("DEBUG") != "" || *debug_flag {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
	}

	logger := promlog.New(&logConfig)
	level.Info(logger).Log("msg", fmt.Sprintf("Starting %s", exporter_name), "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())
	var test []interface{}
	test = append(test, "msg", "test")
	test = append(test, "version", version.BuildContext())
	level.Info(logger).Log(test...)

	cwd, err := os.Getwd()
	if err != nil {
		level.Error(logger).Log("msg", fmt.Sprintf("Error getting cwd(): %s", err))
	} else {
		level.Debug(logger).Log("msg", fmt.Sprintf("cwd %s", cwd))
	}

	// Override the config.file default with the CONFIG environment variable, if set. If the flag is explicitly set, it
	// will end up overriding either.
	if envConfigFile := os.Getenv("CONFIG"); envConfigFile != "" {
		*configFile = envConfigFile
	}

	exporter, err := sql_exporter.NewExporter(*configFile, logger)
	if err != nil {
		level.Error(logger).Log("msg", fmt.Sprintf("Error creating exporter: %s", err))
		os.Exit(1)
	}

	if *dry_run {
		level.Info(logger).Log("msg", "configuration OK.")
		// get the target if defined
		var t sql_exporter.Target
		var err error
		if *target_name != "" {
			t, err = exporter.FindTarget(*target_name)
		} else {
			t, err = exporter.GetFirstTarget()
		}
		if err != nil {
			level.Error(logger).Log("errmsg", err)
			os.Exit(1)
		}
		level.Info(logger).Log("msg", fmt.Sprintf("try to collect target %s.", t.Name()))
		timeout := time.Duration(0)
		configTimeout := time.Duration(exporter.Config().Globals.ScrapeTimeout)

		// If the configured scrape timeout is more restrictive, use that instead.
		if configTimeout > 0 && (timeout <= 0 || configTimeout < timeout) {
			timeout = configTimeout
		}
		var ctx context.Context
		var cancel context.CancelFunc
		if timeout <= 0 {
			ctx = context.Background()
			cancel = func() {}
		} else {
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
		}
		defer cancel()

		gatherer := prometheus.Gatherers{exporter.WithContext(ctx, t)}
		mfs, err := gatherer.Gather()
		if err != nil {
			level.Error(logger).Log("errmsg", fmt.Sprintf("Error gathering metrics: %v", err))
			os.Exit(1)
		}

		level.Info(logger).Log("msg", "collect is OK. Dumping result to stdout.")

		//dump metric to stdout
		enc := expfmt.NewEncoder(os.Stdout, expfmt.FmtText)

		for _, mf := range mfs {
			err := enc.Encode(mf)
			if err != nil {
				level.Error(logger).Log("Errmsg", err)
				break
			}
		}
		if closer, ok := enc.(expfmt.Closer); ok {
			// This in particular takes care of the final "# EOF\n" line for OpenMetrics.
			closer.Close()
		}
		level.Info(logger).Log("msg", "dry-run is over. Exiting.")
		os.Exit(0)
	}

	// Setup and start webserver.
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "OK", http.StatusOK) })
	http.HandleFunc("/", HomeHandlerFunc(*metricsPath))
	http.HandleFunc("/config", ConfigHandlerFunc(*metricsPath, exporter))
	http.Handle(*metricsPath, ExporterHandlerFor(exporter))
	// Expose exporter metrics separately, for debugging purposes.
	http.Handle("/sql_exporter_metrics", promhttp.Handler())

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server")
		os.Exit(1)
	}
}
