package engine

import (
	"testing"

	"github.com/partiri-cloud/message-in-a-box/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestEvaluateWorkflow_AllStepsEnabled(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{Type: "in_app", Order: 0, DefaultEnabled: true},
			{Type: "email", Order: 1, DefaultEnabled: true},
		},
	}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, nil, nil)
	assert.Len(t, result, 2)
	assert.False(t, result[0].Skipped)
	assert.False(t, result[1].Skipped)
}

func TestEvaluateWorkflow_DisabledStep(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{Type: "email", Order: 0, DefaultEnabled: false},
		},
	}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, nil, nil)
	assert.Len(t, result, 1)
	assert.True(t, result[0].Skipped)
	assert.Equal(t, "step disabled by default", result[0].Reason)
}

func TestEvaluateWorkflow_ConditionEq(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.severity", Operator: "eq", Value: "high"},
				},
			},
		},
	}
	payload := map[string]any{"severity": "high"}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, payload, nil)
	assert.False(t, result[0].Skipped)
}

func TestEvaluateWorkflow_ConditionNe(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.severity", Operator: "ne", Value: "low"},
				},
			},
		},
	}
	payload := map[string]any{"severity": "high"}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, payload, nil)
	assert.False(t, result[0].Skipped)
}

func TestEvaluateWorkflow_ConditionContains(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.message", Operator: "contains", Value: "alert"},
				},
			},
		},
	}
	payload := map[string]any{"message": "CPU alert triggered"}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, payload, nil)
	assert.False(t, result[0].Skipped)
}

func TestEvaluateWorkflow_ConditionIn(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.level", Operator: "in", Value: []any{"warn", "error", "critical"}},
				},
			},
		},
	}
	payload := map[string]any{"level": "error"}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, payload, nil)
	assert.False(t, result[0].Skipped)
}

func TestEvaluateWorkflow_ConditionNin(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.level", Operator: "nin", Value: []any{"info", "debug"}},
				},
			},
		},
	}
	payload := map[string]any{"level": "error"}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, payload, nil)
	assert.False(t, result[0].Skipped)
}

func TestEvaluateWorkflow_ConditionNotMet(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.severity", Operator: "eq", Value: "critical"},
				},
			},
		},
	}
	payload := map[string]any{"severity": "low"}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, payload, nil)
	assert.True(t, result[0].Skipped)
	assert.Equal(t, "condition not met", result[0].Reason)
}

func TestEvaluateWorkflow_SubscriberDataResolution(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "subscriber.data.role", Operator: "eq", Value: "admin"},
				},
			},
		},
	}
	sub := &model.Subscriber{
		Data: map[string]any{"role": "admin"},
	}
	result := EvaluateWorkflow(wf, sub, nil, nil)
	assert.False(t, result[0].Skipped)
}

func TestEvaluateWorkflow_StepSeenCondition(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "steps.0.seen", Operator: "eq", Value: false},
				},
			},
		},
	}
	notif := &model.Notification{Seen: false}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, nil, notif)
	assert.False(t, result[0].Skipped)
}

func TestEvaluateWorkflow_MissingField(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.nonexistent", Operator: "eq", Value: "something"},
				},
			},
		},
	}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, map[string]any{}, nil)
	assert.True(t, result[0].Skipped)
}

func TestEvaluateWorkflow_EmptySteps(t *testing.T) {
	wf := &model.Workflow{Steps: []model.WorkflowStep{}}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, nil, nil)
	assert.Empty(t, result)
}

func TestEvaluateWorkflow_MultipleConditions(t *testing.T) {
	wf := &model.Workflow{
		Steps: []model.WorkflowStep{
			{
				Type: "email", Order: 0, DefaultEnabled: true,
				Conditions: []model.StepCondition{
					{Field: "payload.severity", Operator: "eq", Value: "high"},
					{Field: "payload.env", Operator: "eq", Value: "production"},
				},
			},
		},
	}
	// Both conditions met
	payload := map[string]any{"severity": "high", "env": "production"}
	result := EvaluateWorkflow(wf, &model.Subscriber{}, payload, nil)
	assert.False(t, result[0].Skipped)

	// Only first condition met
	payload2 := map[string]any{"severity": "high", "env": "staging"}
	result2 := EvaluateWorkflow(wf, &model.Subscriber{}, payload2, nil)
	assert.True(t, result2[0].Skipped)
}
