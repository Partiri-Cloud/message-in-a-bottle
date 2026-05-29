package config

import (
	"testing"
)

// validKey is a well-formed 64-hex-char (32-byte) key for use in passing tests.
const validKey = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
const validSecret = "this-is-a-valid-hmac-secret-at-least-32-chars!!"
const validAdminSecret = "this-is-a-valid-admin-secret-at-least-32-chars!!"

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", validKey)
}

func TestLoad_Success(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(cfg.CredentialsEncryptionKeyBytes) != 32 {
		t.Errorf("expected CredentialsEncryptionKeyBytes len 32, got %d", len(cfg.CredentialsEncryptionKeyBytes))
	}
	if cfg.SubscriberHMACSecret != validSecret {
		t.Errorf("expected SubscriberHMACSecret to be set")
	}
	if cfg.AdminSecret != validAdminSecret {
		t.Errorf("expected AdminSecret to be set")
	}
}

func TestLoad_MissingAdminSecret(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "")
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", validKey)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing ADMIN_SECRET, got nil")
	}
}

func TestLoad_ShortAdminSecret(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "short")
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", validKey)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short ADMIN_SECRET, got nil")
	}
}

func TestLoad_MissingSubscriberHMACSecret(t *testing.T) {
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", "")
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", validKey)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SUBSCRIBER_HMAC_SECRET, got nil")
	}
}

func TestLoad_ShortSubscriberHMACSecret(t *testing.T) {
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", "tooshort")
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", validKey)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short SUBSCRIBER_HMAC_SECRET, got nil")
	}
}

func TestLoad_MissingCredentialsEncryptionKey(t *testing.T) {
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing CREDENTIALS_ENCRYPTION_KEY, got nil")
	}
}

func TestLoad_OddLengthCredentialsEncryptionKey(t *testing.T) {
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	// Odd number of hex chars — not valid hex
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for odd-length CREDENTIALS_ENCRYPTION_KEY, got nil")
	}
}

func TestLoad_NonHexCredentialsEncryptionKey(t *testing.T) {
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	// 64 chars but contains non-hex characters
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-hex CREDENTIALS_ENCRYPTION_KEY, got nil")
	}
}

func TestLoad_WrongByteLength_TooShort(t *testing.T) {
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	// Valid hex but only 16 bytes (32 hex chars) — not 32 bytes
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", "0102030405060708090a0b0c0d0e0f10")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for wrong-byte-length CREDENTIALS_ENCRYPTION_KEY, got nil")
	}
}

func TestLoad_WrongByteLength_TooLong(t *testing.T) {
	t.Setenv("ADMIN_SECRET", validAdminSecret)
	t.Setenv("SUBSCRIBER_HMAC_SECRET", validSecret)
	// Valid hex but 36 bytes (72 hex chars) — not 32 bytes
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2021222324")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for wrong-byte-length CREDENTIALS_ENCRYPTION_KEY (too long), got nil")
	}
}
