# Security Policy

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Instead, use GitHub's private vulnerability reporting:

1. Go to the [Security Advisories](https://github.com/partiri-cloud/message-in-a-bottle/security/advisories/new) page
2. Click **"Report a vulnerability"**
3. Fill in the details — what you found, how to reproduce it, and the potential impact

You will receive a response within 72 hours. Once the vulnerability is confirmed, we will work on a fix and coordinate a disclosure timeline with you.

## Supported Versions

Only the latest release receives security fixes.

## Security Design Notes

- All third-party provider credentials are encrypted at rest using AES-256-GCM
- API keys are stored as SHA-256 hashes — plaintext keys are never persisted
- Subscriber WebSocket tokens are signed with HMAC-SHA256 and include an expiry
- All secret values (`ADMIN_SECRET`, `CREDENTIALS_ENCRYPTION_KEY`, `SUBSCRIBER_HMAC_SECRET`) must be provided via environment variables — never hardcoded
