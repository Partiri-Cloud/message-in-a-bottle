package engine

import (
	"bytes"
	htmltmpl "html/template"
	"strings"
	texttmpl "text/template"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type TemplateData struct {
	Subscriber TemplateSubscriber
	Payload    map[string]any
}

type TemplateSubscriber struct {
	FirstName string
	LastName  string
	Email     string
}

// RenderTemplate renders using text/template (no HTML escaping). Use for SMS, push, chat, in-app.
func RenderTemplate(tmplStr string, data TemplateData) (string, error) {
	funcMap := texttmpl.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": func(s string) string {
			return cases.Title(language.Und).String(s)
		},
		"default": templateDefault,
	}

	t, err := texttmpl.New("notification").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderHTMLTemplate renders using html/template (contextual HTML escaping). Use for email bodies.
func RenderHTMLTemplate(tmplStr string, data TemplateData) (string, error) {
	funcMap := htmltmpl.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": func(s string) string {
			return cases.Title(language.Und).String(s)
		},
		"default": templateDefault,
	}

	t, err := htmltmpl.New("notification").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func ResolveLocale(localeMap map[string]string, locale, defaultLocale string) string {
	if v, ok := localeMap[locale]; ok {
		return v
	}
	if v, ok := localeMap[defaultLocale]; ok {
		return v
	}
	if v, ok := localeMap["en"]; ok {
		return v
	}
	// Return first available
	for _, v := range localeMap {
		return v
	}
	return ""
}

func templateDefault(defaultVal, actual string) string {
	if actual == "" {
		return defaultVal
	}
	return actual
}
