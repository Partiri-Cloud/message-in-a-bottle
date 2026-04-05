package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseDuration_Minutes(t *testing.T) {
	assert.Equal(t, 5*time.Minute, ParseDuration(5, "minutes"))
}

func TestParseDuration_Hours(t *testing.T) {
	assert.Equal(t, 2*time.Hour, ParseDuration(2, "hours"))
}

func TestParseDuration_Days(t *testing.T) {
	assert.Equal(t, 3*24*time.Hour, ParseDuration(3, "days"))
}

func TestParseDuration_UnknownUnit(t *testing.T) {
	assert.Equal(t, 10*time.Minute, ParseDuration(10, "unknown"))
}

func TestParseDuration_Zero(t *testing.T) {
	assert.Equal(t, time.Duration(0), ParseDuration(0, "minutes"))
}
