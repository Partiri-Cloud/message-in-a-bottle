package handler

import (
	"testing"

	"github.com/partiri-cloud/message-in-a-box/internal/handler/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertSteps_BasicChannel(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "email",
			Order: 0,
			Template: &dto.StepTemplateDTO{
				Subject: map[string]string{"en": "Hello"},
				Body:    map[string]string{"en": "<p>World</p>"},
			},
			DefaultEnabled: true,
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	assert.Equal(t, "email", steps[0].Type)
	assert.Equal(t, 0, steps[0].Order)
	assert.True(t, steps[0].DefaultEnabled)
	assert.NotNil(t, steps[0].Template)
	assert.Equal(t, "Hello", steps[0].Template.Subject["en"])
	assert.Equal(t, "<p>World</p>", steps[0].Template.Body["en"])
}

func TestConvertSteps_DigestConfig(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "digest",
			Order: 1,
			DigestConfig: &dto.DigestConfigDTO{
				Amount:    30,
				Unit:      "minutes",
				DigestKey: "service_id",
			},
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	require.NotNil(t, steps[0].DigestConfig)
	assert.Equal(t, 30, steps[0].DigestConfig.Amount)
	assert.Equal(t, "minutes", steps[0].DigestConfig.Unit)
	assert.Equal(t, "service_id", steps[0].DigestConfig.DigestKey)
}

func TestConvertSteps_DelayConfig(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "delay",
			Order: 2,
			DelayConfig: &dto.DelayConfigDTO{
				Amount: 5,
				Unit:   "minutes",
			},
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	require.NotNil(t, steps[0].DelayConfig)
	assert.Equal(t, 5, steps[0].DelayConfig.Amount)
	assert.Equal(t, "minutes", steps[0].DelayConfig.Unit)
}

func TestConvertSteps_WithConditions(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{
			Type:  "email",
			Order: 0,
			Conditions: []dto.StepConditionDTO{
				{Field: "payload.severity", Operator: "eq", Value: "critical"},
				{Field: "steps.0.seen", Operator: "eq", Value: false},
			},
			DefaultEnabled: true,
		},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	require.Len(t, steps[0].Conditions, 2)
	assert.Equal(t, "payload.severity", steps[0].Conditions[0].Field)
	assert.Equal(t, "eq", steps[0].Conditions[0].Operator)
	assert.Equal(t, "critical", steps[0].Conditions[0].Value)
	assert.Equal(t, false, steps[0].Conditions[1].Value)
}

func TestConvertSteps_GeneratesStepIDs(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{Type: "in_app", Order: 0, DefaultEnabled: true},
		{Type: "email", Order: 1, DefaultEnabled: true},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 2)
	assert.False(t, steps[0].ID.IsZero(), "step 0 should have an ID")
	assert.False(t, steps[1].ID.IsZero(), "step 1 should have an ID")
	assert.NotEqual(t, steps[0].ID, steps[1].ID, "step IDs should be unique")
}

func TestConvertSteps_EmptyOptionalFields(t *testing.T) {
	dtoSteps := []dto.WorkflowStepDTO{
		{Type: "sms", Order: 0, DefaultEnabled: true},
	}

	steps := convertSteps(dtoSteps)
	require.Len(t, steps, 1)
	assert.Nil(t, steps[0].Template)
	assert.Nil(t, steps[0].DigestConfig)
	assert.Nil(t, steps[0].DelayConfig)
	assert.Empty(t, steps[0].Conditions)
}
