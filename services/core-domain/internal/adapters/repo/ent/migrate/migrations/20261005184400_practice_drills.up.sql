-- Frozen practice drill catalog, compiled from motifpath-specs catalogs/practice-drills.yaml by
-- scripts/practice_drills. Each id is the UUID v5 of drill-template/<key> or
-- drill-threshold/<template key>/v<version>. A row already holding one of these ids stops the migration.
INSERT INTO "drill_templates" ("id", "key", "item_kind", "response_type", "timed", "names") VALUES
  ('4c6ffb77-ee8b-5fda-b243-784f4209bc65', 'fretboard_cell:name_the_note', 'fretboard_cell', 'name_the_note', true, '{"en":"Name the note","pt_BR":"Diga a nota"}'),
  ('6e38da0b-df26-5aa8-ab14-3b6640582174', 'fretboard_cell:find_the_note', 'fretboard_cell', 'find_the_note', true, '{"en":"Find the note","pt_BR":"Ache a nota"}'),
  ('eee88cdc-8c03-5246-b798-3ccf6941875f', 'exercise:text_response', 'exercise', 'option_choice', true, '{"en":"Text exercises","pt_BR":"Exercícios de texto"}'),
  ('e11fdaf6-8f60-5348-ab8b-0333cb31c31f', 'exercise:audio_recognition', 'exercise', 'option_choice', true, '{"en":"Listening exercises","pt_BR":"Exercícios de escuta"}'),
  ('bfe9f72d-a2e9-5f43-a774-115b7da268f8', 'exercise:image_recognition', 'exercise', 'option_choice', true, '{"en":"Image recognition exercises","pt_BR":"Exercícios de reconhecimento de imagem"}'),
  ('2ba4103e-ce5d-554f-83a5-eba38a6ed2b6', 'exercise:image_choice', 'exercise', 'option_choice', true, '{"en":"Image choice exercises","pt_BR":"Exercícios de escolha de imagem"}'),
  ('49aed57b-3886-53de-b6d1-4b842acc8ab1', 'exercise:audio_selection', 'exercise', 'option_choice', true, '{"en":"Sound choice exercises","pt_BR":"Exercícios de escolha de som"}');
INSERT INTO "drill_thresholds" ("id", "template_id", "version", "effective_from", "fluent_net_ms", "source", "sessions", "students") VALUES
  ('a2ccf5a4-31e5-5aac-991d-5b17904b4ea8', 'eee88cdc-8c03-5246-b798-3ccf6941875f', 1, '2026-10-01T00:00:00Z', 6000, 'default', 0, 0),
  ('e0364add-6753-58c3-a781-a9589288510e', 'e11fdaf6-8f60-5348-ab8b-0333cb31c31f', 1, '2026-10-01T00:00:00Z', 4000, 'default', 0, 0),
  ('1fa82f45-7f4f-51ee-b974-cf0673eda4a7', 'bfe9f72d-a2e9-5f43-a774-115b7da268f8', 1, '2026-10-01T00:00:00Z', 5000, 'default', 0, 0),
  ('c8d7f798-4ab0-57cb-bf26-5f477154ae87', '2ba4103e-ce5d-554f-83a5-eba38a6ed2b6', 1, '2026-10-01T00:00:00Z', 5000, 'default', 0, 0),
  ('61b55338-dee3-5391-b556-dc8fe121ea6d', '49aed57b-3886-53de-b6d1-4b842acc8ab1', 1, '2026-10-01T00:00:00Z', 4000, 'default', 0, 0);
