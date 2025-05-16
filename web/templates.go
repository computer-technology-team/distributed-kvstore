package web

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var TemplatesFS embed.FS

//go:embed static
var StaticFS embed.FS

type TemplateRenderer interface {
	Render(w http.ResponseWriter, templateName string, data any) error
}

type defaultTemplateRenderer struct {
	templates *template.Template
}

func (r *defaultTemplateRenderer) Render(w http.ResponseWriter, templateName string, data any) error {
	err := r.templates.ExecuteTemplate(w, templateName, data)
	if err != nil {
		return err
	}
	return nil
}

func NewTemplateRenderer() (TemplateRenderer, error) {
	tmpl, err := template.ParseFS(TemplatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &defaultTemplateRenderer{
		templates: tmpl,
	}, nil
}
