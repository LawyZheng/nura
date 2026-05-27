package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/LawyZheng/nura/internal/model"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var funcMap = template.FuncMap{
	"reportTypeCN":     reportTypeCN,
	"derefFloat":       derefFloat,
	"derefInt":         derefInt,
	"locationCN":       locationCN,
	"stoolCN":          stoolCN,
	"companionSymptoms": companionSymptoms,
	"mealTypeCN":       mealTypeCN,
	"seq":              seq,
}

func derefFloat(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%.1f", *f)
}

// pages maps page name → parsed template (layout + page).
var pages map[string]*template.Template

func init() {
	pages = make(map[string]*template.Template)
	pageFiles := []string{"index.html", "dashboard.html", "reports.html", "report_detail.html", "chat.html", "symptoms.html", "meals.html"}

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

func derefInt(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}

func locationCN(loc string) string {
	m := map[string]string{
		"upper_abdomen": "上腹",
		"lower_abdomen": "下腹",
		"left_upper":    "左上腹",
		"right_upper":   "右上腹",
		"navel_area":    "脐周",
	}
	if cn, ok := m[loc]; ok {
		return cn
	}
	return loc
}

func stoolCN(color string) string {
	m := map[string]string{
		"normal": "正常",
		"dark":   "偏深",
		"black":  "黑便",
		"bloody": "血便",
	}
	if cn, ok := m[color]; ok {
		return cn
	}
	return color
}

func companionSymptoms(sl *model.SymptomLog) string {
	var parts []string
	if sl.Bloating {
		parts = append(parts, "腹胀")
	}
	if sl.Nausea {
		parts = append(parts, "恶心")
	}
	if sl.AcidReflux {
		parts = append(parts, "反酸")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "、")
}

func mealTypeCN(mt string) string {
	m := map[string]string{
		"breakfast": "早餐",
		"lunch":     "午餐",
		"dinner":    "晚餐",
		"snack":     "加餐",
	}
	if cn, ok := m[mt]; ok {
		return cn
	}
	return mt
}

func seq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i + 1
	}
	return s
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
