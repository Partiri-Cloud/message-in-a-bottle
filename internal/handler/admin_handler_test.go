package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── resolvePermissions unit tests ───────────────────────────────────────────

func TestResolvePermissions_NilSlice_ReturnsAll(t *testing.T) {
	perms, err := resolvePermissions(nil)
	require.NoError(t, err)
	assert.Equal(t, allPermissions(), perms)
}

func TestResolvePermissions_EmptySlice_ReturnsAll(t *testing.T) {
	perms, err := resolvePermissions([]string{})
	require.NoError(t, err)
	assert.Equal(t, allPermissions(), perms)
}

func TestResolvePermissions_Wildcard_ReturnsAll(t *testing.T) {
	perms, err := resolvePermissions([]string{"*"})
	require.NoError(t, err)
	assert.Equal(t, allPermissions(), perms)
}

func TestResolvePermissions_SingleValidPermission_ReturnsThatSubset(t *testing.T) {
	perms, err := resolvePermissions([]string{"notifications:read"})
	require.NoError(t, err)
	require.Len(t, perms, 1)
	assert.Equal(t, "notifications:read", perms[0])
}

func TestResolvePermissions_MultipleValidPermissions_ReturnsExactSubset(t *testing.T) {
	requested := []string{"workflows:read", "workflows:write", "notifications:trigger"}
	perms, err := resolvePermissions(requested)
	require.NoError(t, err)
	assert.Equal(t, requested, perms)
}

func TestResolvePermissions_UnknownPermission_Returns400Error(t *testing.T) {
	_, err := resolvePermissions([]string{"notifications:read", "unicorn:fly"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown permission: unicorn:fly")
}

func TestResolvePermissions_TemplatesSend_IsGrantable(t *testing.T) {
	perms, err := resolvePermissions([]string{"templates:send"})
	require.NoError(t, err)
	require.Len(t, perms, 1)
	assert.Equal(t, "templates:send", perms[0])
}

// ─── allPermissions completeness ─────────────────────────────────────────────

func TestAllPermissions_ContainsTemplatesSend(t *testing.T) {
	all := allPermissions()
	found := false
	for _, p := range all {
		if p == "templates:send" {
			found = true
			break
		}
	}
	assert.True(t, found, "allPermissions() must include templates:send")
}

func TestAllPermissions_NoDuplicates(t *testing.T) {
	all := allPermissions()
	seen := make(map[string]struct{}, len(all))
	for _, p := range all {
		_, dup := seen[p]
		assert.False(t, dup, "duplicate permission entry: %s", p)
		seen[p] = struct{}{}
	}
}
