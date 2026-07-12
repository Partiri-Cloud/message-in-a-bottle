package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
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

func TestIsControlStep(t *testing.T) {
	assert.True(t, IsControlStep("delay"))
	assert.True(t, IsControlStep("digest"))
	assert.False(t, IsControlStep("email"))
	assert.False(t, IsControlStep("in_app"))
	assert.False(t, IsControlStep(""))
}

func TestBuildChannelDeliveries_SkipsControlSteps(t *testing.T) {
	steps := []model.WorkflowStep{
		{Type: "email"},
		{Type: "delay"},
		{Type: "sms"},
		{Type: "digest"},
		{Type: "in_app"},
	}

	channels := BuildChannelDeliveries(steps)

	assert.Equal(t, []model.ChannelDelivery{
		{Channel: "email", Status: "pending"},
		{Channel: "sms", Status: "pending"},
		{Channel: "in_app", Status: "pending"},
	}, channels)
}

func TestBuildChannelDeliveries_Empty(t *testing.T) {
	assert.Empty(t, BuildChannelDeliveries(nil))
	assert.Empty(t, BuildChannelDeliveries([]model.WorkflowStep{{Type: "delay"}}))
}
