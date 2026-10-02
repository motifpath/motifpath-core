-- Frozen catalog instruments. Each id is the UUID v5 of instrument/<key>, the same in every environment.
-- a row already holding one of these ids stops the migration.
INSERT INTO "instruments" ("id", "names", "family", "string_count", "tuning", "default_voice_id") VALUES
  -- guitar
  ('6ea2d087-ab9c-59dc-9657-8546025414d2', '{"en":"Acoustic guitar","pt_BR":"Violão"}', 'fretted', 6, '["E2","A2","D3","G3","B3","E4"]', 'acoustic-guitar'),
  -- electric-guitar
  ('e6fac4f3-7d52-5f46-8f44-4de1b239ebdd', '{"en":"Electric guitar","pt_BR":"Guitarra elétrica"}', 'fretted', 6, '["E2","A2","D3","G3","B3","E4"]', 'acoustic-guitar'),
  -- electric-bass
  ('14fe11ad-efdb-589a-b713-2e81ec041cbe', '{"en":"Electric bass","pt_BR":"Contrabaixo elétrico"}', 'fretted', 4, '["E1","A1","D2","G2"]', 'electric-bass');
