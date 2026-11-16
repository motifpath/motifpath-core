-- Ownership history is append-only: a correction adds a revision, and the
-- database itself refuses to change or remove one already recorded.
CREATE FUNCTION "content_ownerships_append_only"() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'content ownership revisions are append-only (% refused)', TG_OP
    USING ERRCODE = 'insufficient_privilege';
END;
$$;
CREATE TRIGGER "content_ownerships_append_only"
  BEFORE UPDATE OR DELETE ON "content_ownerships"
  FOR EACH ROW EXECUTE FUNCTION "content_ownerships_append_only"();
CREATE TRIGGER "content_ownerships_no_truncate"
  BEFORE TRUNCATE ON "content_ownerships"
  FOR EACH STATEMENT EXECUTE FUNCTION "content_ownerships_append_only"();
