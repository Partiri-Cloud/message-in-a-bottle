package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExtractSendGridMessageID_HeaderMissing pins the panic fix: a 2xx SendGrid
// response is not guaranteed to carry X-Message-Id, so a missing key must yield
// an empty message ID instead of indexing a nil slice.
func TestExtractSendGridMessageID_HeaderMissing(t *testing.T) {
	id := extractSendGridMessageID(map[string][]string{})
	assert.Empty(t, id)
}

func TestExtractSendGridMessageID_HeaderPresentButEmpty(t *testing.T) {
	id := extractSendGridMessageID(map[string][]string{"X-Message-Id": {}})
	assert.Empty(t, id)
}

func TestExtractSendGridMessageID_HeaderPresent(t *testing.T) {
	id := extractSendGridMessageID(map[string][]string{"X-Message-Id": {"abc123"}})
	assert.Equal(t, "abc123", id)
}
