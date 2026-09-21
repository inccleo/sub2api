package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesktopAuthConfigOptIn(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("DESKTOP_AUTH_ENABLED", "false")
	cfg, err := Load()
	require.NoError(t, err)
	require.False(t, cfg.DesktopAuth.Enabled)
	t.Setenv("DESKTOP_AUTH_ENABLED", "true")
	cfg, err = Load()
	require.NoError(t, err)
	require.True(t, cfg.DesktopAuth.Enabled)
}
