-- Add the fork-specific Kimi platform without overwriting operator settings.

UPDATE channel_monitor_v2_config
SET
    platforms = platforms || '[{"platform":"kimi","enabled":true,"models":[]}]'::jsonb,
    version = version + 1,
    updated_at = NOW()
WHERE id = 1
  AND NOT EXISTS (
      SELECT 1
      FROM jsonb_array_elements(platforms) AS platform_config
      WHERE platform_config ->> 'platform' = 'kimi'
  );
