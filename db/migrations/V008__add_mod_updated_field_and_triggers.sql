ALTER TABLE mods DROP COLUMN IF EXISTS updated;

ALTER TABLE mods ADD COLUMN updated timestamp with time zone NOT NULL DEFAULT NOW();

DROP TRIGGER IF EXISTS auto_updated ON mods;
DROP TRIGGER IF EXISTS auto_updated ON mod_releases;

DROP FUNCTION IF EXISTS auto_updated_trigger;

CREATE FUNCTION auto_updated_trigger() RETURNS trigger LANGUAGE plpgsql AS
$$
BEGIN
    NEW.updated := NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER auto_updated BEFORE INSERT OR UPDATE ON mods
    FOR EACH ROW EXECUTE FUNCTION auto_updated_trigger();

CREATE TRIGGER auto_updated BEFORE INSERT OR UPDATE ON mod_releases
    FOR EACH ROW EXECUTE FUNCTION auto_updated_trigger();
