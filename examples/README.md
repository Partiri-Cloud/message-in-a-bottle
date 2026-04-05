# Example Templates

Ready-to-use notification templates for common use cases. Each JSON file can be posted directly to the API.

## Prerequisites

- A running Message in a Bottle instance
- An API key with appropriate permissions (see [DEPLOYMENT.md](../DEPLOYMENT.md))

## Usage

### 1. Create a workflow

```bash
curl -X POST http://localhost:3000/api/v1/workflows \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d @examples/workflows/welcome.json
```

### 2. Trigger it

```bash
curl -X POST http://localhost:3000/api/v1/events/trigger \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d @examples/triggers/welcome.json
```

### 3. Transactional templates

Create a standalone template, then send it directly:

```bash
# Create
curl -X POST http://localhost:3000/api/v1/templates \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d @examples/templates/verification-code.json

# Send
curl -X POST http://localhost:3000/api/v1/templates/verification-code/send \
  -H "Authorization: ApiKey <your-api-key>" \
  -H "Content-Type: application/json" \
  -d @examples/triggers/verification-code.json
```

## Workflow Examples

| File | Channels | Locales | Use case |
|------|----------|---------|----------|
| [welcome.json](workflows/welcome.json) | email, in_app | en, fr, pt, de, it, es | New user onboarding |
| [order-confirmation.json](workflows/order-confirmation.json) | email, in_app, sms | en, fr, pt, de, it, es | E-commerce order with delay + conditional SMS |
| [password-reset.json](workflows/password-reset.json) | email | en | Security-focused single step |
| [comment-mention.json](workflows/comment-mention.json) | in_app, push | en | Social/collaboration notifications |
| [invoice-receipt.json](workflows/invoice-receipt.json) | email | en, fr, pt, de, it, es | Billing receipt with HTML table |
| [activity-digest.json](workflows/activity-digest.json) | email | en | Batched activity over 1-hour window |

## Transactional Template Examples

| File | Channel | Locales | Use case |
|------|---------|---------|----------|
| [verification-code.json](templates/verification-code.json) | email | en | Email verification code |
| [shipping-update.json](templates/shipping-update.json) | email | en, pt | Order shipping status |

## Trigger Payloads

Each file in [triggers/](triggers/) shows the payload shape for its matching workflow or template. The payload keys correspond to `{{.Payload.<key>}}` variables used in the templates.

## Template Syntax Reference

Templates use [Go html/template](https://pkg.go.dev/html/template) syntax.

### Variables

| Variable | Description |
|----------|-------------|
| `{{.Subscriber.FirstName}}` | Subscriber's first name |
| `{{.Subscriber.LastName}}` | Subscriber's last name |
| `{{.Subscriber.Email}}` | Subscriber's email address |
| `{{.Payload.<key>}}` | Any value passed in the trigger payload |

### Helper functions

| Function | Example | Result |
|----------|---------|--------|
| `upper` | `{{upper .Payload.name}}` | `HELLO` |
| `lower` | `{{lower .Payload.name}}` | `hello` |
| `title` | `{{title .Payload.name}}` | `Hello` |
| `default` | `{{default "there" .Subscriber.FirstName}}` | Uses `"there"` if FirstName is empty |

### Channel template fields

Each channel type uses different template fields:

| Channel | Template fields |
|---------|----------------|
| `email` | `subject` + `body` (HTML) |
| `sms` | `content` (plain text) |
| `push` | `subject` + `body` |
| `in_app` | `subject` + `content` |
| `chat` (Slack/Teams) | `content` |

### Localization

Subject, body, and content fields are locale maps:

```json
{
  "subject": {
    "en": "Welcome, {{.Subscriber.FirstName}}!",
    "pt": "Bem-vindo, {{.Subscriber.FirstName}}!"
  }
}
```

Locale resolution order:
1. `locale` from the trigger request
2. Subscriber's `locale` field
3. Template's `defaultLocale`
4. `"en"` as final fallback
