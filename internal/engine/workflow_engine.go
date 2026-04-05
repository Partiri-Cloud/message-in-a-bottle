package engine

import (
	"fmt"
	"strings"

	"github.com/partiri/message-in-a-bottle/internal/model"
)

type PlannedStep struct {
	StepIndex int
	Step      model.WorkflowStep
	Skipped   bool
	Reason    string
}

func EvaluateWorkflow(wf *model.Workflow, subscriber *model.Subscriber, payload map[string]any, notification *model.Notification) []PlannedStep {
	var planned []PlannedStep

	for i, step := range wf.Steps {
		ps := PlannedStep{StepIndex: i, Step: step}

		if !step.DefaultEnabled {
			ps.Skipped = true
			ps.Reason = "step disabled by default"
			planned = append(planned, ps)
			continue
		}

		// Evaluate conditions
		allMet := true
		for _, cond := range step.Conditions {
			if !evaluateCondition(cond, subscriber, payload, notification) {
				allMet = false
				break
			}
		}
		if !allMet {
			ps.Skipped = true
			ps.Reason = "condition not met"
		}

		planned = append(planned, ps)
	}

	return planned
}

func evaluateCondition(cond model.StepCondition, subscriber *model.Subscriber, payload map[string]any, notification *model.Notification) bool {
	val := resolveField(cond.Field, subscriber, payload, notification)
	return compareValues(val, cond.Operator, cond.Value)
}

func resolveField(field string, subscriber *model.Subscriber, payload map[string]any, notification *model.Notification) any {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) < 2 {
		return nil
	}

	switch parts[0] {
	case "subscriber":
		return resolveSubscriberField(parts[1], subscriber)
	case "payload":
		return resolveMapField(parts[1], payload)
	case "steps":
		return resolveStepField(parts[1], notification)
	}
	return nil
}

func resolveSubscriberField(path string, sub *model.Subscriber) any {
	parts := strings.SplitN(path, ".", 2)
	switch parts[0] {
	case "data":
		if len(parts) > 1 {
			return resolveMapField(parts[1], sub.Data)
		}
		return sub.Data
	case "locale":
		return sub.Locale
	case "email":
		return sub.Email
	case "phone":
		return sub.Phone
	}
	return nil
}

func resolveMapField(path string, data map[string]any) any {
	parts := strings.Split(path, ".")
	current := any(data)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

func resolveStepField(path string, notification *model.Notification) any {
	if notification == nil {
		return nil
	}
	// Format: "0.seen", "0.read"
	parts := strings.SplitN(path, ".", 2)
	if len(parts) < 2 {
		return nil
	}
	// For now, step status is tracked on the notification
	switch parts[1] {
	case "seen":
		return notification.Seen
	case "read":
		return notification.Read
	}
	return nil
}

func compareValues(actual any, operator string, expected any) bool {
	switch operator {
	case "eq":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "ne":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", expected))
	case "in":
		if arr, ok := expected.([]any); ok {
			for _, item := range arr {
				if fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", item) {
					return true
				}
			}
		}
		return false
	case "nin":
		if arr, ok := expected.([]any); ok {
			for _, item := range arr {
				if fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", item) {
					return false
				}
			}
		}
		return true
	}
	return false
}
