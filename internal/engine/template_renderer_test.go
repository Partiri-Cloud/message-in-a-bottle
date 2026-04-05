package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderTemplate_SimpleVariable(t *testing.T) {
	data := TemplateData{
		Payload: map[string]any{"Name": "World"},
	}
	result, err := RenderTemplate("Hello {{.Payload.Name}}", data)
	require.NoError(t, err)
	assert.Equal(t, "Hello World", result)
}

func TestRenderTemplate_SubscriberFields(t *testing.T) {
	data := TemplateData{
		Subscriber: TemplateSubscriber{FirstName: "Sarah", LastName: "Chen"},
	}
	result, err := RenderTemplate("Hi {{.Subscriber.FirstName}} {{.Subscriber.LastName}}", data)
	require.NoError(t, err)
	assert.Equal(t, "Hi Sarah Chen", result)
}

func TestRenderTemplate_UpperFunc(t *testing.T) {
	data := TemplateData{
		Payload: map[string]any{"Name": "hello"},
	}
	result, err := RenderTemplate("{{upper .Payload.Name}}", data)
	require.NoError(t, err)
	assert.Equal(t, "HELLO", result)
}

func TestRenderTemplate_LowerFunc(t *testing.T) {
	data := TemplateData{
		Payload: map[string]any{"Name": "HELLO"},
	}
	result, err := RenderTemplate("{{lower .Payload.Name}}", data)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestRenderTemplate_DefaultFunc(t *testing.T) {
	data := TemplateData{
		Payload: map[string]any{},
	}
	result, err := RenderTemplate(`{{default "fallback" ""}}`, data)
	require.NoError(t, err)
	assert.Equal(t, "fallback", result)
}

func TestRenderTemplate_InvalidSyntax(t *testing.T) {
	data := TemplateData{}
	_, err := RenderTemplate("{{.Bad", data)
	assert.Error(t, err)
}

func TestResolveLocale_ExactMatch(t *testing.T) {
	m := map[string]string{"en": "Hello", "es": "Hola", "fr": "Bonjour"}
	assert.Equal(t, "Hola", ResolveLocale(m, "es", "en"))
}

func TestResolveLocale_DefaultFallback(t *testing.T) {
	m := map[string]string{"en": "Hello", "fr": "Bonjour"}
	assert.Equal(t, "Hello", ResolveLocale(m, "es", "en"))
}

func TestResolveLocale_EnglishFallback(t *testing.T) {
	m := map[string]string{"en": "Hello", "de": "Hallo"}
	// Requested "pt" not found, default "de" found → returns "de" (default takes priority over "en")
	assert.Equal(t, "Hallo", ResolveLocale(m, "pt", "de"))
}

func TestResolveLocale_FirstAvailable(t *testing.T) {
	m := map[string]string{"ja": "こんにちは"}
	result := ResolveLocale(m, "pt", "de")
	assert.Equal(t, "こんにちは", result)
}

func TestResolveLocale_EmptyMap(t *testing.T) {
	assert.Equal(t, "", ResolveLocale(map[string]string{}, "en", "en"))
}
