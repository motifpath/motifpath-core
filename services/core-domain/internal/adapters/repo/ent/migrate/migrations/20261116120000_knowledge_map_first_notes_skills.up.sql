-- The major-scale and chord-chart transposition skills added to the knowledge
-- map after the first production install, which had already applied
-- baseline_reference_data. Their ids are the catalog's fixed ids. A key or id
-- already in use stops the migration.
INSERT INTO "knowledge_nodes" ("id", "kind", "key", "names", "parent_id") VALUES
  ('47e46616-8511-513a-8753-600699077de0', 'skill', 'build-major-scale', '{"en":"Build the major scale from any note","pt_BR":"Montar a escala maior a partir de qualquer nota"}', '3a51c665-a147-5e56-b232-fff2a4b073be'),
  ('08268009-43c4-5ecd-bd9b-6b08ae90524a', 'skill', 'transpose-chord-chart', '{"en":"Transpose a chord chart to another key","pt_BR":"Passar uma cifra para outro tom"}', '4e9f89c8-1fb7-5726-9717-db4e513ec0ac');
INSERT INTO "knowledge_edges" ("id", "type", "from_id", "to_id", "level") VALUES
  ('22f8255b-aa04-5284-a196-e83fe7e9f15a', 'applies', '47e46616-8511-513a-8753-600699077de0', '0cb8cd99-2f95-5dc7-8737-ac8c3a2cfc20', NULL),
  ('2d0b4d3e-6985-52b9-9743-870d5b419422', 'applies', '47e46616-8511-513a-8753-600699077de0', 'a0405b41-0709-5763-a8c9-294c348fd6a2', NULL),
  ('85c7be1d-6d9a-516e-a180-eb482ce21a0d', 'applies', '47e46616-8511-513a-8753-600699077de0', '1a273528-6153-538d-905e-d28d8ff9f6fb', NULL),
  ('389001e7-c92f-5c78-9fec-5cfee9d5f177', 'applies', '47e46616-8511-513a-8753-600699077de0', 'ba9a4de3-0d31-5545-b467-0391999a3e2c', NULL),
  ('0abcf31e-ace9-514c-b70c-3aa703048a7e', 'applies', '08268009-43c4-5ecd-bd9b-6b08ae90524a', '7588a2f4-709f-518e-bdd7-ecac428f70ff', NULL),
  ('510fa68b-9698-5732-95fe-17009d5ef37e', 'applies', '08268009-43c4-5ecd-bd9b-6b08ae90524a', 'ba9a4de3-0d31-5545-b467-0391999a3e2c', NULL),
  ('1d1de973-c5fc-59ba-9e9b-901c4e2f77fe', 'applies', '08268009-43c4-5ecd-bd9b-6b08ae90524a', 'f8917f4a-20fc-50a5-8ecd-df5ea85b87f9', NULL),
  ('d3c3796b-e8be-5293-8f7c-b66092c95941', 'requires', '47e46616-8511-513a-8753-600699077de0', 'e63e430c-3569-5916-8ad5-850a4380a79d', 'accurate'),
  ('85d5cf8e-90c1-5015-90d7-79638fade912', 'requires', '08268009-43c4-5ecd-bd9b-6b08ae90524a', '47e46616-8511-513a-8753-600699077de0', 'accurate'),
  ('1904bb9a-0e58-5587-8779-86b563b8ce0c', 'requires', '08268009-43c4-5ecd-bd9b-6b08ae90524a', '1c2bbf22-4cd1-5687-ad46-10e4f74bb2ec', 'accurate');
