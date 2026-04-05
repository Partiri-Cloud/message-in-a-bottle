package dto

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(query string) *gin.Context {
	req := httptest.NewRequest("GET", "/?"+query, nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c
}

func TestParsePagination_Defaults(t *testing.T) {
	c := newTestContext("")
	page, limit := ParsePagination(c)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, limit)
}

func TestParsePagination_ValidValues(t *testing.T) {
	c := newTestContext("page=3&limit=50")
	page, limit := ParsePagination(c)
	assert.Equal(t, 3, page)
	assert.Equal(t, 50, limit)
}

func TestParsePagination_ZeroPage(t *testing.T) {
	c := newTestContext("page=0")
	page, _ := ParsePagination(c)
	assert.Equal(t, 1, page)
}

func TestParsePagination_NegativeLimit(t *testing.T) {
	c := newTestContext("limit=-1")
	_, limit := ParsePagination(c)
	assert.Equal(t, 20, limit)
}

func TestParsePagination_ExceedsMax(t *testing.T) {
	c := newTestContext("limit=200")
	_, limit := ParsePagination(c)
	assert.Equal(t, 20, limit)
}

func TestParsePagination_NonNumeric(t *testing.T) {
	c := newTestContext("page=abc&limit=xyz")
	page, limit := ParsePagination(c)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, limit)
}
