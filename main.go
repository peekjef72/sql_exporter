package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"syscall"
	"time"

	_ "net/http/pprof"

	// "github.com/peekjef72/db2_exporter"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

var (
	metricsPath    = kingpin.Flag("web.telemetry-path", "Path under which to expose collector's internal metrics.").Default("/metrics").String()
	configFile     = kingpin.Flag("config.file", fmt.Sprintf("%s Exporter configuration file.", exporter_name)).Short('c').Default("config/config.yml").String()
	debug_flag     = kingpin.Flag("debug", "debug connection checks.").Short('d').Default("false").Bool()
	dry_run        = kingpin.Flag("dry-run", "check exporter configuration file and try to collect a target then exit.").Short('n').Default("false").Bool()
	target_name    = kingpin.Flag("target", "In dry-run mode specify the target name, else ignored.").Short('t').String()
	auth_key       = kingpin.Flag("auth.key", "In dry-run mode specify the auth_key to use, else ignored.").Short('a').String()
	collector_name = kingpin.Flag("collector", "Specify the collector name restriction to collect, replace the collector_names set for each target.").Short('o').String()
	toolkitFlags   = kingpinflag.AddFlags(kingpin.CommandLine, metricsPublishingPort)
	logConfig      = promlog.Config{}
)

func init() {
	prometheus.MustRegister(version.NewCollector(exporter_name))
}

const (
	OpEgals = 1
	OpMatch = 2
)

type route struct {
	path    string
	regex   *regexp.Regexp
	handler http.HandlerFunc
}

func newRoute(op int, path string, handler http.HandlerFunc) *route {
	if op == OpEgals {
		return &route{path, nil, handler}
	} else if op == OpMatch {
		return &route{"", regexp.MustCompile("^" + path + "$"), handler}

	} else {
		return nil
	}

}
func BuildHandler(exporter Exporter) http.Handler {
	var routes = []*route{
		newRoute(OpEgals, "/healthz", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "OK", http.StatusOK) }),
		newRoute(OpEgals, "/", HomeHandlerFunc(*metricsPath, exporter)),
		newRoute(OpEgals, "/status", StatusHandlerFunc(*metricsPath, exporter)),
		newRoute(OpEgals, "/config", ConfigHandlerFunc(*metricsPath, exporter)),
		newRoute(OpEgals, "/targets", TargetsHandlerFunc(*metricsPath, exporter)),
		newRoute(OpEgals, *metricsPath, func(w http.ResponseWriter, r *http.Request) { ExporterHandlerFor(exporter).ServeHTTP(w, r) }),
		// Expose exporter metrics separately, for debugging purposes.
		newRoute(OpEgals, "/httpapi_exporter_metrics", func(w http.ResponseWriter, r *http.Request) { promhttp.Handler().ServeHTTP(w, r) }),

		// pprof handle
		newRoute(OpMatch, "/debug/.+", http.DefaultServeMux.ServeHTTP),
		// newRoute("/debug/pprof/cmdline", pprof.Cmdline),
		// newRoute("/debug/pprof/profile", pprof.Profile),
		// newRoute("/debug/pprof/symbol", pprof.Symbol),
		// newRoute("/debug/pprof/trace", pprof.Trace),
		// newRoute("default", func(w http.ResponseWriter, r *http.Request) { http.Handler() .ServeHTTP(w, r) }),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, route := range routes {
			if route == nil {
				continue
			}
			if route.regex != nil {
				if route.regex.MatchString(req.URL.Path) {
					route.handler(w, req)
					return
				}
			} else if req.URL.Path == route.path {
				route.handler(w, req)
				return
			}
		}
		// http.DefaultServeMux.ServeHTTP(w, req)
		// h, _ := http.DefaultServeMux.Handler(req)
		// if h != nil {
		// 			h.ServeHTTP(w, req)
		// } else {
		err := fmt.Errorf("not found")
		HandleError(http.StatusNotFound, err, *metricsPath, exporter, w, req)
		// }
	})

}

func main() {

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

	// Override the config.file default with the CONFIG environment variable, if set. If the flag is explicitly set, it
	// will end up overriding either.
	if envConfigFile := os.Getenv(configEnvName); envConfigFile != "" {
		*configFile = envConfigFile
	}

	exporter, err := NewExporter(*configFile, logger, *collector_name)
	if err != nil {
		level.Error(logger).Log("msg", fmt.Sprintf("Error creating exporter: %s", err))
		os.Exit(1)
	}

	exporter.SetLogLevel(logConfig.Level.String())

	if *dry_run {
		level.Info(logger).Log("msg", "configuration OK.")
		// get the target if defined
		var t Target
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
		if *auth_key != "" {
			t.SetSymbol("auth_key", *auth_key)
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

	exporter.SetStartTime(time.Now())
	exporter.SetReloadTime(time.Now())

	user2 := make(chan os.Signal, 1)
	signal.Notify(user2, syscall.SIGUSR2)
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-user2:
				exporter.IncreaseLogLevel()
			case <-hup:
				level.Info(logger).Log("msg", "file reloading.")
				if err := exporter.ReloadConfig(); err != nil {
					level.Error(logger).Log("msg", fmt.Sprintf("reload err: %s.", err))
				} else {
					level.Info(logger).Log("msg", "file reloaded.")
				}
			}
		}
	}()

	srvc := make(chan struct{})
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Setup and start webserver.
		server := &http.Server{
			Handler: BuildHandler(exporter),
		}
		if err := web.ListenAndServe(server, toolkitFlags, logger); err != nil {
			level.Error(logger).Log("err", err)
			os.Exit(1)
		}

		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "OK", http.StatusOK) })
		http.HandleFunc("/", HomeHandlerFunc(*metricsPath, exporter))
		http.HandleFunc("/config", ConfigHandlerFunc(*metricsPath, exporter))
		http.HandleFunc("/status", StatusHandlerFunc(*metricsPath, exporter))
		http.HandleFunc("/targets", TargetsHandlerFunc(*metricsPath, exporter))
		http.Handle(*metricsPath, ExporterHandlerFor(exporter))
		// Expose exporter metrics separately, for debugging purposes.
		http.Handle("/sql_exporter_metrics", promhttp.Handler())
	}()

	for {
		select {
		case <-term:
			level.Info(logger).Log("msg", "Received SIGTERM, exiting gracefully...")
			os.Exit(0)
		case <-srvc:
			os.Exit(1)
		}
	}
}
