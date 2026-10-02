-- add the "electric-bass" voice: every third semitone from E1 (MIDI 28), the 4-string bass's lowest string, up to G4 (67)
INSERT INTO "voices" ("id", "names", "family", "pitches", "attribution") VALUES
  ('electric-bass', '{"en": "Electric bass", "pt_BR": "Contrabaixo elétrico"}', 'fretted',
   (SELECT jsonb_agg(p ORDER BY p) FROM generate_series(28, 67, 3) AS p),
   'Electric bass samples from tonejs-instruments by Nicholaus Brosowsky, CC BY 3.0');
