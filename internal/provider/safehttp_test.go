package provider

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errFrag string
	}{
		{
			name:    "allow valid Slack URL",
			url:     "https://hooks.slack.com/services/T00000000/B00000000/XXXX",
			wantErr: false,
		},
		{
			name:    "allow valid MS Teams URL",
			url:     "https://myorg.webhook.office.com/webhookb2/xxxxx",
			wantErr: false,
		},
		{
			name:    "allow MS Teams URL with deep subdomain",
			url:     "https://tenant.webhook.office.com/webhookb2/yyy",
			wantErr: false,
		},
		{
			name:    "reject http scheme for Slack",
			url:     "http://hooks.slack.com/services/x",
			wantErr: true,
			errFrag: "https",
		},
		{
			name:    "reject link-local IP (IMDS)",
			url:     "https://169.254.169.254/latest/meta-data/",
			wantErr: true,
			errFrag: "blocked",
		},
		{
			name:    "reject localhost by name",
			url:     "https://localhost/",
			wantErr: true,
			errFrag: "not in the allowed list",
		},
		{
			name:    "reject private RFC-1918 10.x",
			url:     "https://10.0.0.1/",
			wantErr: true,
			errFrag: "blocked",
		},
		{
			name:    "reject loopback 127.0.0.1",
			url:     "https://127.0.0.1/",
			wantErr: true,
			errFrag: "blocked",
		},
		{
			name:    "reject non-allowlisted HTTPS host",
			url:     "https://evil.com/",
			wantErr: true,
			errFrag: "not in the allowed list",
		},
		{
			name:    "reject private RFC-1918 192.168.x",
			url:     "https://192.168.1.1/hook",
			wantErr: true,
			errFrag: "blocked",
		},
		{
			name:    "reject ULA IPv6",
			url:     "https://[fd00::1]/hook",
			wantErr: true,
			errFrag: "blocked",
		},
		{
			name:    "reject lookalike suffix — must not match evilwebhook.office.com",
			url:     "https://evilwebhook.office.com/x",
			wantErr: true,
			errFrag: "not in the allowed list",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateWebhookURL(tc.url)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errFrag != "" {
					assert.Contains(t, err.Error(), tc.errFrag)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGuardedDial_RejectsPrivateIPLiteral(t *testing.T) {
	// guardedDial must refuse to connect to a private IP literal without
	// making any real network connection.
	_, err := guardedDial(context.Background(), "tcp", "10.0.0.1:80")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked address")
}

func TestGuardedDial_RejectsLoopback(t *testing.T) {
	_, err := guardedDial(context.Background(), "tcp", "127.0.0.1:6379")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked address")
}

func TestGuardedDial_RejectsLinkLocal(t *testing.T) {
	_, err := guardedDial(context.Background(), "tcp", "169.254.169.254:80")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked address")
}

func TestCheckIP_AllCategories(t *testing.T) {
	cases := []struct {
		ip      string
		wantErr bool
	}{
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		{"0.0.0.0", true},
		{"fd00::1", true},
		{"fe80::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}

	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		require.NotNil(t, ip, "could not parse %s", tc.ip)
		err := checkIP(ip)
		if tc.wantErr {
			assert.Error(t, err, "expected block for %s", tc.ip)
		} else {
			assert.NoError(t, err, "expected allow for %s", tc.ip)
		}
	}
}
