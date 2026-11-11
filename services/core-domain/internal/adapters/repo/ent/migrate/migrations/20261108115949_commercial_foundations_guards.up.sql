-- Commercial evidence is append-only: the database itself refuses to change
-- or remove an audit entry, whatever the application does. Removal will only
-- ever happen through a verified archive purge, which replaces this guard.
CREATE FUNCTION "commercial_audit_entries_append_only"() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'commercial audit entries are append-only (% refused)', TG_OP
    USING ERRCODE = 'insufficient_privilege';
END;
$$;
CREATE TRIGGER "commercial_audit_entries_append_only"
  BEFORE UPDATE OR DELETE ON "commercial_audit_entries"
  FOR EACH ROW EXECUTE FUNCTION "commercial_audit_entries_append_only"();
CREATE TRIGGER "commercial_audit_entries_no_truncate"
  BEFORE TRUNCATE ON "commercial_audit_entries"
  FOR EACH STATEMENT EXECUTE FUNCTION "commercial_audit_entries_append_only"();

-- The environment's single activation row, at revision 0. It is the lock that
-- fences new intents behind a committed activation change.
INSERT INTO "commercial_activations" ("id", "revision") VALUES (1, 0);

-- Every registered flag starts OFF, not open to all users, with no subjects.
INSERT INTO "commercial_flags" ("flag", "enabled", "all_users", "subject_ids") VALUES
  ('content_ownership', false, false, '[]'),
  ('content_versions', false, false, '[]'),
  ('strapi_rights', false, false, '[]'),
  ('commercial_validation', false, false, '[]'),
  ('entitlement_resolution', false, false, '[]'),
  ('access_grants', false, false, '[]'),
  ('progress_retention', false, false, '[]'),
  ('authorization_enforcement', false, false, '[]'),
  ('plus_catalog', false, false, '[]'),
  ('plus_webhooks', false, false, '[]'),
  ('plus_billing', false, false, '[]'),
  ('plus_checkout', false, false, '[]'),
  ('teacher_paid_paths', false, false, '[]'),
  ('path_purchase', false, false, '[]'),
  ('motifpath_purchase', false, false, '[]'),
  ('teacher_settlement', false, false, '[]'),
  ('private_beta', false, false, '[]');
