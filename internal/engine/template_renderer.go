package engine

import (
	"bytes"
	"html/template"
	"strings"
)

type TemplateData struct {
	Subscriber TemplateSubscriber
	Payload    map[string]any
	Env        TemplateEnv
}

type TemplateSubscriber struct {
	FirstName string
	LastName  string
	Email     string
}

type TemplateEnv struct {
	Name string
}

func RenderTemplate(tmplStr string, data TemplateData) (string, error) {
	funcMap := template.FuncMap{
		"upper":   strings.ToUpper,
		"lower":   strings.ToLower,
		"title":   strings.Title,
		"default": templateDefault,
	}

	t, err := template.New("notification").Funcs(funcMap).Parse(tmplStr)
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
