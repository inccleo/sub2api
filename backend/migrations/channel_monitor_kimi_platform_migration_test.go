package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorKimiPlatformMigration(t *testing.T) {
	content, err := FS.ReadFile("221_channel_monitor_v2_add_kimi.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, `platforms = platforms || '[{"platform":"kimi","enabled":true,"models":[]}]'::jsonb`)
	require.Contains(t, sql, "FROM jsonb_array_elements(platforms)")
	require.Contains(t, sql, "platform_config ->> 'platform' = 'kimi'")
	require.NotContains(t, sql, "SET platforms = $platforms$")
}
