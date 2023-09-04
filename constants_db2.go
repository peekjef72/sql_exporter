//go:build db2 && !oracle && !mssql

package main

var (
	exporter_namespace = "db2"
)

const (
	metricsPublishingPort = ":9161"
	exporter_name         = "db2_exporter"
	configEnvName         = "DB2_CONFIG"

	docsUrl   = "https://github.com/peekjef72/db2_exporter#readme"
	templates = `
    {{ define "page" -}}
      <html>
      <head>
        <title>Prometheus DB2 Exporter</title>
        <style type="text/css">
          body { margin: 0; font-family: "Helvetica Neue", Helvetica, Arial, sans-serif; font-size: 14px; line-height: 1.42857143; color: #333; background-color: #fff; }
          .navbar { display: flex; background-color: #222; margin: 0; border-width: 0 0 1px; border-style: solid; border-color: #080808; }
          .navbar > * { margin: 0; padding: 15px; }
          .navbar * { line-height: 20px; color: #9d9d9d; }
          .navbar a { text-decoration: none; }
          .navbar a:hover, .navbar a:focus { color: #fff; }
          .navbar-header { font-size: 18px; }
          body > * { margin: 15px; padding: 0; }
          pre { padding: 10px; font-size: 13px; background-color: #f5f5f5; border: 1px solid #ccc; }
          h1, h2 { font-weight: 500; }
          a { color: #337ab7; }
          a:hover, a:focus { color: #23527c; }
        </style>
      </head>
      <body>
        <div class="navbar">
          <div class="navbar-header"><a href="/">Prometheus DB2 Exporter</a></div>
          <div><a href="{{ .MetricsPath }}">Metrics</a></div>
          <div><a href="/config">Configuration</a></div>
          <div><a href="/debug/pprof">Profiling</a></div>
          <div><a href="{{ .DocsUrl }}">Help</a></div>
        </div>
        {{template "content" .}}
      </body>
      </html>
    {{- end }}

    {{ define "content.home" -}}
      <p>This is a <a href="{{ .DocsUrl }}">Prometheus DB2 Exporter</a> instance.
        You are probably looking for its <a href="{{ .MetricsPath }}">metrics</a> handler.</p>
    {{- end }}

    {{ define "content.config" -}}
      <h2>Configuration</h2>
      <pre>{{ .Config }}</pre>
    {{- end }}

    {{ define "content.error" -}}
      <h2>Error</h2>
      <pre>{{ .Err }}</pre>
    {{- end }}
    `
)
