-- The system languages and the playback voices. No generator produces these
-- rows; they are carried over unchanged from the migrations that first
-- installed them.

-- en, pt_BR, and the literal "any" marking language-agnostic content. Each id
-- is fixed — UUID v5 of language/<code> under the MotifPath catalog
-- namespace — so it is the same in every environment.
INSERT INTO "languages" ("id", "code", "name") VALUES
  ('5a118a9d-1ed7-57d5-b607-395681a862e1', 'en', 'English'),
  ('802e1939-95ed-5d17-8b2a-e5dddfe60853', 'pt_BR', 'Portuguese (Brazil)'),
  ('7d7dda1c-f802-50fd-a30e-45f605fc71b0', 'any', 'Language-agnostic');

-- Each voice is sampled every third semitone across its instrument's range:
-- the acoustic guitar from E2 (MIDI 40), the piano from A0 (21), and the
-- 4-string electric bass from E1 (28) up to G4 (67).
INSERT INTO "voices" ("id", "names", "family", "pitches", "attribution") VALUES
  ('acoustic-guitar', '{"en": "Acoustic guitar", "pt_BR": "Violão"}', 'fretted',
   (SELECT jsonb_agg(p ORDER BY p) FROM generate_series(40, 88, 3) AS p),
   'Acoustic guitar samples from tonejs-instruments by Nicholaus Brosowsky, CC BY 3.0'),
  ('piano', '{"en": "Piano", "pt_BR": "Piano"}', 'keyboard',
   (SELECT jsonb_agg(p ORDER BY p) FROM generate_series(21, 108, 3) AS p),
   'Piano samples from tonejs-instruments by Nicholaus Brosowsky, CC BY 3.0'),
  ('electric-bass', '{"en": "Electric bass", "pt_BR": "Contrabaixo elétrico"}', 'fretted',
   (SELECT jsonb_agg(p ORDER BY p) FROM generate_series(28, 67, 3) AS p),
   'Electric bass samples from tonejs-instruments by Nicholaus Brosowsky, CC BY 3.0');
