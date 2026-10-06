-- Frozen fretboard cell ranges, compiled from motifpath-specs catalogs/practice-drills.yaml by
-- scripts/practice_drills. Each id is the UUID v5 of fretboard-cells/<skill key>/<layout key>.
-- Every string and fret in a range is a generated practice item of its skill.
INSERT INTO "fretboard_cell_ranges" ("id", "skill_id", "layout_instrument_id", "strings", "from_fret", "to_fret") VALUES
  ('14ecec38-6d3c-53a4-b30c-a3fd3f54bf63', '94fe4e24-2440-535f-b99d-08dac70d9961', '6ea2d087-ab9c-59dc-9657-8546025414d2', '[6,5]', 0, 11),
  ('d4229cfd-a71e-5fdf-8bf8-c853fd739ed1', '94fe4e24-2440-535f-b99d-08dac70d9961', '14fe11ad-efdb-589a-b713-2e81ec041cbe', '[4,3]', 0, 11),
  ('08606f0f-7141-5c88-807a-8be30872334c', '8583c6f4-2460-5acf-9578-2ab98436198e', '6ea2d087-ab9c-59dc-9657-8546025414d2', '[4,3,2,1]', 0, 11),
  ('d4443c3f-a319-5dbd-ba2b-5db01a7da873', '8583c6f4-2460-5acf-9578-2ab98436198e', '14fe11ad-efdb-589a-b713-2e81ec041cbe', '[2,1]', 0, 11);
