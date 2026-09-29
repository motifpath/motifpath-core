-- create "voices" table
CREATE TABLE "voices" ("id" character varying NOT NULL, "names" jsonb NOT NULL, "family" character varying NOT NULL, "pitches" jsonb NOT NULL, "attribution" character varying NOT NULL, PRIMARY KEY ("id"));
-- the platform's first voices, sampled every 3 semitones so a note is never re-pitched by more than one
INSERT INTO "voices" ("id", "names", "family", "pitches", "attribution") VALUES
  ('acoustic-guitar', '{"en": "Acoustic guitar", "pt_BR": "Violão"}', 'fretted',
   (SELECT jsonb_agg(p ORDER BY p) FROM generate_series(40, 88, 3) AS p),
   'Acoustic guitar samples from tonejs-instruments by Nicholaus Brosowsky, CC BY 3.0'),
  ('piano', '{"en": "Piano", "pt_BR": "Piano"}', 'keyboard',
   (SELECT jsonb_agg(p ORDER BY p) FROM generate_series(21, 108, 3) AS p),
   'Piano samples from tonejs-instruments by Nicholaus Brosowsky, CC BY 3.0');
-- modify "instruments" table: every instrument is played by a voice of its family
ALTER TABLE "instruments" ADD COLUMN "default_voice_id" character varying NULL;
UPDATE "instruments" SET "default_voice_id" = CASE "family" WHEN 'fretted' THEN 'acoustic-guitar' ELSE 'piano' END;
ALTER TABLE "instruments" ALTER COLUMN "default_voice_id" SET NOT NULL, ADD CONSTRAINT "instruments_voices_default_voice" FOREIGN KEY ("default_voice_id") REFERENCES "voices" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- each open string gets its octave. Only the standard tunings are known, and any other stops the migration rather than guess
UPDATE "instruments" SET "tuning" = CASE "tuning"
    WHEN '["E", "A", "D", "G", "B", "E"]'::jsonb THEN '["E2", "A2", "D3", "G3", "B3", "E4"]'::jsonb
    WHEN '["B", "E", "A", "D", "G", "B", "E"]'::jsonb THEN '["B1", "E2", "A2", "D3", "G3", "B3", "E4"]'::jsonb
    WHEN '["E", "A", "D", "G"]'::jsonb THEN '["E1", "A1", "D2", "G2"]'::jsonb
    WHEN '["B", "E", "A", "D", "G"]'::jsonb THEN '["B0", "E1", "A1", "D2", "G2"]'::jsonb
  END
  WHERE "family" = 'fretted';
-- a fretted instrument whose tuning matched none of them is now left without one: adding this check then fails the migration, until that tuning is given octaves by hand
ALTER TABLE "instruments" ADD CONSTRAINT "instruments_fretted_tuning_has_octaves" CHECK ("family" <> 'fretted' OR "tuning" IS NOT NULL);
ALTER TABLE "instruments" DROP CONSTRAINT "instruments_fretted_tuning_has_octaves";
-- modify "diagrams" table: a diagram's key, meter, tempo and playback steps
ALTER TABLE "diagrams" ADD COLUMN "mode" character varying NULL, ADD COLUMN "tempo_bpm" bigint NULL, ADD COLUMN "time_signature_beats" bigint NOT NULL DEFAULT 4, ADD COLUMN "time_signature_beat_value" bigint NOT NULL DEFAULT 4, ADD COLUMN "sequence" jsonb NOT NULL DEFAULT '[]';
-- no diagram plays yet: its old sequence indices were the order the editor's positions were clicked in, not a sequence an author chose
ALTER TABLE "diagrams" ALTER COLUMN "sequence" DROP DEFAULT;
-- modify "positions" table
ALTER TABLE "positions" DROP COLUMN "sequence_index";
