-- The drill templates added after the first production install, which had
-- already applied baseline_reference_data. Their ids are the catalog's fixed
-- ids, so a database that has them already, installed while the baseline
-- still carried them, is left as it is.
INSERT INTO "drill_templates" ("id", "key", "item_kind", "response_type", "rung", "second_way", "comes_back", "timed", "names") VALUES
  ('f3e74d03-ed82-514f-99e0-8f7ed46f27d3', 'fretboard_cell:find_every_instance', 'fretboard_cell', 'multi_tap', 'know', NULL, false, false, '{"en":"Find every instance","pt_BR":"Ache todas as posições"}'),
  ('448900eb-6410-554f-9e2e-dd96621008f6', 'guide_tone:tap_the_third', 'guide_tone', 'timed_taps', 'target', NULL, false, false, '{"en":"Guide tones","pt_BR":"Notas-guia"}')
ON CONFLICT ("id") DO NOTHING;
