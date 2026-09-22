-- seed system rows: en, pt_BR, and the literal "any" marking language-agnostic content (ADR-024)
INSERT INTO "languages" ("id", "code", "name") VALUES
  (gen_random_uuid(), 'en', 'English'),
  (gen_random_uuid(), 'pt_BR', 'Portuguese (Brazil)'),
  (gen_random_uuid(), 'any', 'Language-agnostic');
