package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"io"

	"github.com/pion/webrtc/v4"
)

//go:embed templates/*.html
var templateFiles embed.FS

type templateRenderer struct {
	templates *template.Template
}

type indexPageData struct {
	LinearLimit  float64
	AngularLimit float64
	RobotID      string
	ICEServers   []webrtc.ICEServer
	RelayOnly    bool
}

func (d indexPageData) ICEServersJSON() template.JS {
	if d.ICEServers == nil {
		return template.JS("[]")
	}
	encoded, _ := json.Marshal(d.ICEServers)
	return template.JS(encoded)
}

func newTemplateRenderer() (*templateRenderer, error) {
	templates, err := template.ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &templateRenderer{templates: templates}, nil
}

func (r *templateRenderer) render(w io.Writer, name string, data any) error {
	return r.templates.ExecuteTemplate(w, name, data)
}
