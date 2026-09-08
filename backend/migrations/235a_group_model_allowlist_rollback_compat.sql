-- Preserve the pre-0.2.2 column so an automatic binary rollback can still
-- read and update group model configuration after migration 235 has run.
DO $$
DECLARE
    has_legacy BOOLEAN;
    has_current BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'groups'
          AND column_name = 'models_list_config'
    ) INTO has_legacy;
    SELECT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'groups'
          AND column_name = 'model_allowlist'
    ) INTO has_current;

    IF has_legacy AND NOT has_current THEN
        ALTER TABLE groups ADD COLUMN model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb;
        UPDATE groups SET model_allowlist = models_list_config;
    ELSIF has_current AND NOT has_legacy THEN
        ALTER TABLE groups ADD COLUMN models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb;
        UPDATE groups SET models_list_config = model_allowlist;
    ELSIF NOT has_legacy AND NOT has_current THEN
        ALTER TABLE groups ADD COLUMN models_list_config JSONB NOT NULL DEFAULT '{}'::jsonb;
        ALTER TABLE groups ADD COLUMN model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb;
    END IF;

    IF EXISTS (
        SELECT 1 FROM groups
        WHERE models_list_config IS DISTINCT FROM model_allowlist
    ) THEN
        RAISE EXCEPTION 'groups model allowlist compatibility columns contain conflicting data';
    END IF;
END
$$;

CREATE OR REPLACE FUNCTION sync_group_model_allowlist_columns()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.models_list_config IS DISTINCT FROM NEW.model_allowlist THEN
            IF NEW.models_list_config = '{}'::jsonb THEN
                NEW.models_list_config := NEW.model_allowlist;
            ELSIF NEW.model_allowlist = '{}'::jsonb THEN
                NEW.model_allowlist := NEW.models_list_config;
            ELSE
                RAISE EXCEPTION 'conflicting models_list_config and model_allowlist values';
            END IF;
        END IF;
        RETURN NEW;
    END IF;

    IF NEW.models_list_config IS DISTINCT FROM OLD.models_list_config
       AND NEW.model_allowlist IS DISTINCT FROM OLD.model_allowlist THEN
        IF NEW.models_list_config IS DISTINCT FROM NEW.model_allowlist THEN
            RAISE EXCEPTION 'conflicting models_list_config and model_allowlist values';
        END IF;
    ELSIF NEW.models_list_config IS DISTINCT FROM OLD.models_list_config THEN
        NEW.model_allowlist := NEW.models_list_config;
    ELSIF NEW.model_allowlist IS DISTINCT FROM OLD.model_allowlist THEN
        NEW.models_list_config := NEW.model_allowlist;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS groups_model_allowlist_compat_insert ON groups;
CREATE TRIGGER groups_model_allowlist_compat_insert
    BEFORE INSERT ON groups
    FOR EACH ROW EXECUTE FUNCTION sync_group_model_allowlist_columns();

DROP TRIGGER IF EXISTS groups_model_allowlist_compat_update ON groups;
CREATE TRIGGER groups_model_allowlist_compat_update
    BEFORE UPDATE OF models_list_config, model_allowlist ON groups
    FOR EACH ROW EXECUTE FUNCTION sync_group_model_allowlist_columns();

COMMENT ON COLUMN groups.models_list_config IS
    'Legacy rollback-compatible mirror of model_allowlist';
