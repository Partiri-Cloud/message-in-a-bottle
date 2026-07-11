package model

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The channel table is the single enumeration every other package iterates. If a
// field is added to ChannelPrefs without a row here, masking and DTO conversion
// silently leave it false — which is exactly the class of bug the table exists to
// prevent, so pin the two together.
func TestChannelTableCoversEveryChannelPrefsField(t *testing.T) {
	typ := reflect.TypeOf(ChannelPrefs{})
	require.Equal(t, typ.NumField(), len(channelFields), "every ChannelPrefs field needs a row in channelFields")

	byBSONField := map[string]bool{}
	for _, f := range channelFields {
		byBSONField[f.bsonField] = true
	}
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("bson")
		assert.True(t, byBSONField[tag], "ChannelPrefs.%s (bson:%q) is missing from channelFields", typ.Field(i).Name, tag)
	}
}

func TestChannelPrefs_GetSet(t *testing.T) {
	var p ChannelPrefs

	for _, name := range ChannelNames() {
		v, ok := p.Get(name)
		assert.True(t, ok, "%s should be a known channel", name)
		assert.False(t, v)

		require.True(t, p.Set(name, true))
		v, _ = p.Get(name)
		assert.True(t, v, "%s should be enabled after Set", name)
	}

	assert.Equal(t, AllChannelsEnabled(), p, "setting every channel is the all-enabled value")

	_, ok := p.Get("carrier_pigeon")
	assert.False(t, ok)
	assert.False(t, p.Set("carrier_pigeon", true))
}

// And is the global opt-out mask: it can silence a channel the defaults had on,
// never enable one they had off.
func TestChannelPrefs_And(t *testing.T) {
	defaults := ChannelPrefs{Email: true, InApp: true}
	mask := ChannelPrefs{Email: true, SMS: true}

	got := defaults.And(mask)

	assert.True(t, got.Email, "on in both")
	assert.False(t, got.InApp, "the mask silences it")
	assert.False(t, got.SMS, "the mask cannot enable what the defaults have off")
	assert.Equal(t, defaults, defaults.And(AllChannelsEnabled()), "the all-enabled mask changes nothing")
}

func TestChannelBSONField(t *testing.T) {
	field, ok := ChannelBSONField("in_app")
	require.True(t, ok)
	assert.Equal(t, "inApp", field, "the delivery name and the stored field differ")

	_, ok = ChannelBSONField("carrier_pigeon")
	assert.False(t, ok)
}
