-- seed system rows: en, pt_BR, and the literal "any" marking language-agnostic content (ADR-024).
-- Each id is fixed — UUID v5 of language/<code> under the MotifPath catalog namespace — so it is the same in every environment.
INSERT INTO "languages" ("id", "code", "name") VALUES
  ('5a118a9d-1ed7-57d5-b607-395681a862e1', 'en', 'English'),
  ('802e1939-95ed-5d17-8b2a-e5dddfe60853', 'pt_BR', 'Portuguese (Brazil)'),
  ('7d7dda1c-f802-50fd-a30e-45f605fc71b0', 'any', 'Language-agnostic');
