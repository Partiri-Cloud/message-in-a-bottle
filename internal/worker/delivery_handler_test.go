package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/partiri-cloud/message-in-a-bottle/internal/model"
	"github.com/partiri-cloud/message-in-a-bottle/internal/repository"
	"github.com/partiri-cloud/message-in-a-bottle/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestIsTransactional_TopLevelFieldTrue(t *testing.T) {
	assert.True(t, isTransactional(DeliveryPayload{Transactional: true}))
}

func TestIsTransactional_TopLevelFieldFalse(t *testing.T) {
	assert.False(t, isTransactional(DeliveryPayload{Transactional: false}))
}

// TestIsTransactional_WorkflowPayloadMapCannotSpoof pins the security fix: for the
// workflow/trigger path, payload.Payload is the tenant's raw trigger event data and
// must never be trusted to gate transactional routing. Only the dedicated
// top-level Transactional field, set solely by TemplateService.Send, may do so.
func TestIsTransactional_WorkflowPayloadMapCannotSpoof(t *testing.T) {
	payload := DeliveryPayload{
		Transactional: false,
		Payload:       map[string]any{"__transactional": true},
	}
	assert.False(t, isTransactional(payload))
}

func TestRecipientForChannel_Email(t *testing.T) {
	sub := &model.Subscriber{Email: "user@example.com"}
	to, err := recipientForChannel(sub, "email")
	assert.NoError(t, err)
	assert.Equal(t, "user@example.com", to)
}

func TestRecipientForChannel_EmailMissing(t *testing.T) {
	sub := &model.Subscriber{}
	_, err := recipientForChannel(sub, "email")
	assert.Error(t, err)
}

func TestRecipientForChannel_SMS(t *testing.T) {
	sub := &model.Subscriber{Phone: "+15551234567"}
	to, err := recipientForChannel(sub, "sms")
	assert.NoError(t, err)
	assert.Equal(t, "+15551234567", to)
}

func TestRecipientForChannel_SMSMissing(t *testing.T) {
	sub := &model.Subscriber{}
	_, err := recipientForChannel(sub, "sms")
	assert.Error(t, err)
}

func TestRecipientForChannel_Slack(t *testing.T) {
	sub := &model.Subscriber{Channels: model.SubscriberChannels{Slack: model.SlackConfig{WebhookURL: "https://hooks.slack.com/x"}}}
	to, err := recipientForChannel(sub, "slack")
	assert.NoError(t, err)
	assert.Equal(t, "https://hooks.slack.com/x", to)
}

func TestRecipientForChannel_SlackMissing(t *testing.T) {
	sub := &model.Subscriber{}
	_, err := recipientForChannel(sub, "slack")
	assert.Error(t, err)
}

func TestRecipientForChannel_MSTeams(t *testing.T) {
	sub := &model.Subscriber{Channels: model.SubscriberChannels{MSTeams: model.MSTeamsConfig{WebhookURL: "https://teams.example/x"}}}
	to, err := recipientForChannel(sub, "ms_teams")
	assert.NoError(t, err)
	assert.Equal(t, "https://teams.example/x", to)
}

func TestRecipientForChannel_MSTeamsMissing(t *testing.T) {
	sub := &model.Subscriber{}
	_, err := recipientForChannel(sub, "ms_teams")
	assert.Error(t, err)
}

func TestRecipientForChannel_InAppUnsupported(t *testing.T) {
	sub := &model.Subscriber{}
	_, err := recipientForChannel(sub, "in_app")
	assert.Error(t, err)
}

func TestRecipientForChannel_PushUnsupported(t *testing.T) {
	sub := &model.Subscriber{}
	_, err := recipientForChannel(sub, "push")
	assert.Error(t, err)
}

func TestRecipientForChannel_UnknownChannelUnsupported(t *testing.T) {
	sub := &model.Subscriber{}
	_, err := recipientForChannel(sub, "carrier_pigeon")
	assert.Error(t, err)
}

func TestBuildTransactionalSendOptions_MapsFields(t *testing.T) {
	opts := buildTransactionalSendOptions("user@example.com", "Hello", "World")
	assert.Equal(t, "user@example.com", opts.To)
	assert.Equal(t, "Hello", opts.Subject)
	assert.Equal(t, "World", opts.Content)
	assert.NotNil(t, opts.Metadata)
}

func TestBuildTransactionalSendOptions_EmptyFields(t *testing.T) {
	opts := buildTransactionalSendOptions("user@example.com", "", "")
	assert.Equal(t, "user@example.com", opts.To)
	assert.Empty(t, opts.Subject)
	assert.Empty(t, opts.Content)
}

func TestStepAt_InRange(t *testing.T) {
	steps := []model.WorkflowStep{{Type: "email"}, {Type: "sms"}}
	wf := &model.Workflow{Steps: steps}

	step, err := stepAt(wf, 1)
	require.NoError(t, err)
	assert.Equal(t, "sms", step.Type)
}

func TestStepAt_IndexEqualToLength(t *testing.T) {
	wf := &model.Workflow{Steps: []model.WorkflowStep{{Type: "email"}}}

	_, err := stepAt(wf, 1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, asynq.SkipRetry))
}

func TestStepAt_IndexBeyondLength(t *testing.T) {
	wf := &model.Workflow{Steps: []model.WorkflowStep{{Type: "email"}}}

	_, err := stepAt(wf, 5)
	require.Error(t, err)
	assert.True(t, errors.Is(err, asynq.SkipRetry))
}

func TestStepAt_NegativeIndex(t *testing.T) {
	wf := &model.Workflow{Steps: []model.WorkflowStep{{Type: "email"}}}

	_, err := stepAt(wf, -1)
	require.Error(t, err)
	assert.True(t, errors.Is(err, asynq.SkipRetry))
}

func TestSkipRetryError_SatisfiesErrorsIs(t *testing.T) {
	err := skipRetryError(errors.New("boom"))
	assert.True(t, errors.Is(err, asynq.SkipRetry))
	assert.Contains(t, err.Error(), "boom")
}

func TestNextRetryDelay_IncrementsAndBacksOff(t *testing.T) {
	nextAttempt, backoff, ok := nextRetryDelay(0)
	assert.True(t, ok)
	assert.Equal(t, 1, nextAttempt)
	assert.Equal(t, 120*time.Second, backoff) // 30s base * 4^1
}

func TestNextRetryDelay_LastAllowedAttempt(t *testing.T) {
	nextAttempt, backoff, ok := nextRetryDelay(MaxRetries - 1)
	assert.True(t, ok)
	assert.Equal(t, MaxRetries, nextAttempt)
	assert.Equal(t, 1920*time.Second, backoff) // 30s base * 4^3
}

func TestNextRetryDelay_ExhaustedAtMaxRetries(t *testing.T) {
	nextAttempt, backoff, ok := nextRetryDelay(MaxRetries)
	assert.False(t, ok)
	assert.Equal(t, MaxRetries, nextAttempt)
	assert.Equal(t, time.Duration(0), backoff)
}

func TestNextRetryDelay_ExhaustedBeyondMaxRetries(t *testing.T) {
	_, _, ok := nextRetryDelay(MaxRetries + 5)
	assert.False(t, ok)
}

// TestProcessTransactional_UnsupportedChannel_DeadLettersImmediately exercises the
// real processTransactional path (not a re-implementation) for an unsupported
// channel, asserting the returned error satisfies errors.Is(err, asynq.SkipRetry)
// so asynq dead-letters the task instead of retrying it. DeliveryHandler holds
// concrete repos, so this needs a real activityRepo (the only repo touched before
// the early return); it uses the same testutil.SetupTestDB convention as the
// repository package and skips if MongoDB is unavailable.
func TestProcessTransactional_UnsupportedChannel_DeadLettersImmediately(t *testing.T) {
	db, cleanup := testutil.SetupTestDB(t)
	defer cleanup()

	h := &DeliveryHandler{
		activityRepo:  repository.NewActivityRepository(db),
		retentionDays: 1,
	}

	sub := &model.Subscriber{}
	payload := DeliveryPayload{Channel: "in_app", Transactional: true}

	err := h.processTransactional(context.Background(), bson.NewObjectID(), bson.NewObjectID(), sub, payload)
	require.Error(t, err)
	assert.True(t, errors.Is(err, asynq.SkipRetry))
}
