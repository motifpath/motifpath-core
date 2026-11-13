-- The heard chord quality drill and its first fluent time, added after the
-- first production install, which had already applied baseline_reference_data.
-- Their ids are the catalog's fixed ids, so a database that has them already
-- is left as it is.
INSERT INTO "drill_templates" ("id", "key", "item_kind", "response_type", "rung", "second_way", "comes_back", "timed", "names") VALUES
  ('75d75102-aedc-5eb6-924e-b4fc1a6a065c', 'heard_chord_quality:chord_quality_by_ear', 'heard_chord_quality', 'generated_choice', 'hear', NULL, true, true, '{"en":"Hear the chord quality","pt_BR":"Ouça o tipo de acorde"}')
ON CONFLICT ("id") DO NOTHING;
INSERT INTO "drill_thresholds" ("id", "template_id", "version", "effective_from", "fluent_net_ms", "source", "sessions", "students") VALUES
  ('678dbc5c-d046-50b6-8d49-29cd909e97dd', '75d75102-aedc-5eb6-924e-b4fc1a6a065c', 1, '2026-11-12T00:00:00Z', 2500, 'default', 0, 0)
ON CONFLICT ("id") DO NOTHING;
