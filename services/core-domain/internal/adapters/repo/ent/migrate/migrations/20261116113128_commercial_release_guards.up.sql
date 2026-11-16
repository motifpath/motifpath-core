-- Commercial releases are immutable evidence: a new manifest adds a release,
-- and the database itself refuses to change or remove one already stored.
CREATE FUNCTION "commercial_releases_append_only"() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'commercial releases are append-only (% refused)', TG_OP
    USING ERRCODE = 'insufficient_privilege';
END;
$$;
CREATE TRIGGER "commercial_releases_append_only"
  BEFORE UPDATE OR DELETE ON "commercial_releases"
  FOR EACH ROW EXECUTE FUNCTION "commercial_releases_append_only"();
CREATE TRIGGER "commercial_releases_no_truncate"
  BEFORE TRUNCATE ON "commercial_releases"
  FOR EACH STATEMENT EXECUTE FUNCTION "commercial_releases_append_only"();
