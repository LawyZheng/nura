package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var funcMap = template.FuncMap{
	"reportTypeCN": reportTypeCN,
}

// pages maps page name → parsed template (layout + page).
var pages map[string]*template.Template

func init() {
	pages = make(map[string]*template.Template)
	pageFiles := []string{"index.html", "dashboard.html", "reports.html", "report_detail.html"}

	for _, pf := range pageFiles {
		t := template.Must(
			template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/"+pf),
		)
		pages[pf] = t
	}
}

// StaticHandler returns an http.Handler that serves embedded static files.
func StaticHandler() http.Handler {
	return http.FileServer(http.FS(staticFS))
}

// StaticFS returns the embedded static file system for use with gin.StaticFS.
func StaticFS() http.FileSystem {
	sub, _ := fs.Sub(staticFS, "static")
	return http.FS(sub)
}

// Render executes a page template using the "layout" block.
func Render(w io.Writer, page string, data any) error {
	t, ok := pages[page]
	if !ok {
		return fmt.Errorf("template %q not found", page)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

func reportTypeCN(rt string) string {
	m := map[string]string{
		"gastroscopy":        "胃镜",
		"hp_breath":          "HP呼气试验",
		"hp_antibody":        "HP抗体",
		"blood_routine":      "血常规",
		"liver_function":     "肝功能",
		"kidney_function":    "肾功能",
		"stool_routine":      "便常规",
		"stool_occult_blood": "便潜血",
		"general_checkup":    "综合体检",
		"unknown":            "未知",
	}
	if cn, ok := m[rt]; ok {
		return cn
	}
	return rt
}
