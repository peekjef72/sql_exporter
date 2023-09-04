package main

import (
	"fmt"
	"html/template"
	"net/http"
)

type tdata struct {
	MetricsPath string
	DocsUrl     string

	// `/config` only
	Config string

	// `/error` only
	Err error
}

var (
	allTemplates   = template.Must(template.New("").Parse(templates))
	homeTemplate   = pageTemplate("home")
	configTemplate = pageTemplate("config")
	errorTemplate  = pageTemplate("error")
)

func pageTemplate(name string) *template.Template {
	pageTemplate := fmt.Sprintf(`{{define "content"}}{{template "content.%s" .}}{{end}}{{template "page" .}}`, name)
	return template.Must(template.Must(allTemplates.Clone()).Parse(pageTemplate))
}

// HomeHandlerFunc is the HTTP handler for the home page (`/`).
func HomeHandlerFunc(metricsPath string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		homeTemplate.Execute(w, &tdata{
			MetricsPath: metricsPath,
			DocsUrl:     docsUrl,
		})
	}
}

// ConfigHandlerFunc is the HTTP handler for the `/config` page. It outputs the configuration marshaled in YAML format.
func ConfigHandlerFunc(metricsPath string, exporter Exporter) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		config, err := exporter.Config().YAML()
		if err != nil {
			HandleError(err, metricsPath, w, r)
			return
		}
		configTemplate.Execute(w, &tdata{
			MetricsPath: metricsPath,
			DocsUrl:     docsUrl,
			Config:      string(config),
		})
	}
}

// HandleError is an error handler that other handlers defer to in case of error. It is important to not have written
// anything to w before calling HandleError(), or the 500 status code won't be set (and the content might be mixed up).
func HandleError(err error, metricsPath string, w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	errorTemplate.Execute(w, &tdata{
		MetricsPath: metricsPath,
		DocsUrl:     docsUrl,
		Err:         err,
	})
}
