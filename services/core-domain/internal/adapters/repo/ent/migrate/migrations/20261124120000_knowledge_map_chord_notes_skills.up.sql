-- The skills of the theory paths after sh-first-notes (MOT-200): finding
-- intervals, the notes of major, minor and seventh chords, the chords of a
-- major key, and intervals on the fretboard. Also the requires edges from the
-- installed skills these paths open. Rendered by scripts/knowledge_map from
-- motifpath-specs catalogs/knowledge-map.yaml. Ids are the catalog's fixed ids.
-- A key or id already in use stops the migration.
INSERT INTO "knowledge_nodes" ("id", "kind", "key", "names", "parent_id") VALUES
  ('deec0ac4-05ac-50fb-8c5d-14eee1ae3ce4', 'skill', 'find-intervals', '{"en":"Find intervals from any note","pt_BR":"Achar intervalos a partir de qualquer nota"}', '3a51c665-a147-5e56-b232-fff2a4b073be'),
  ('ac133b26-b06b-567b-9f0f-54751e0efe25', 'skill', 'find-chord-notes', '{"en":"Find the notes of major and minor chords","pt_BR":"Achar as notas de acordes maiores e menores"}', '4e9f89c8-1fb7-5726-9717-db4e513ec0ac'),
  ('c398925a-59af-52e7-9be4-b9eb7812ff56', 'skill', 'find-chords-of-key', '{"en":"Find the chords of a major key","pt_BR":"Achar os acordes de um tom maior"}', '4e9f89c8-1fb7-5726-9717-db4e513ec0ac'),
  ('7d7512c9-85c0-5b48-89fb-263423afb7f5', 'skill', 'find-seventh-chord-notes', '{"en":"Find the notes of seventh chords","pt_BR":"Achar as notas de acordes com sétima"}', '4e9f89c8-1fb7-5726-9717-db4e513ec0ac'),
  ('31e6d82d-2724-5917-af32-78b5520bf921', 'skill', 'find-intervals-on-fretboard', '{"en":"Find intervals on the fretboard","pt_BR":"Achar intervalos no braço"}', 'e3586050-c804-57d0-a0c7-5537eaada368');
INSERT INTO "knowledge_node_instruments" ("knowledge_node_id", "instrument_id", "linked_at") VALUES
  ('31e6d82d-2724-5917-af32-78b5520bf921', '6ea2d087-ab9c-59dc-9657-8546025414d2', '2026-11-24T00:00:00Z'),
  ('31e6d82d-2724-5917-af32-78b5520bf921', 'e6fac4f3-7d52-5f46-8f44-4de1b239ebdd', '2026-11-24T00:00:00Z'),
  ('31e6d82d-2724-5917-af32-78b5520bf921', '14fe11ad-efdb-589a-b713-2e81ec041cbe', '2026-11-24T00:00:00Z');
INSERT INTO "knowledge_edges" ("id", "type", "from_id", "to_id", "level") VALUES
  ('961cfc43-d421-5e73-bbc1-7e07132ca16f', 'applies', '31e6d82d-2724-5917-af32-78b5520bf921', '4ab6b60b-c56f-51fb-9d77-bdbc44f0f80a', NULL),
  ('43e56fdb-7206-573c-954c-cbb26ad20b7d', 'applies', '31e6d82d-2724-5917-af32-78b5520bf921', '7659fb15-aa77-5baf-97db-2e55360f2235', NULL),
  ('05273b74-2e0c-5102-808b-4d5225d9bac4', 'applies', 'ac133b26-b06b-567b-9f0f-54751e0efe25', '0ef50d79-6f73-5a89-be9c-4ff76c8c1020', NULL),
  ('a19949f5-5811-53c6-a729-473d1e236e72', 'applies', 'ac133b26-b06b-567b-9f0f-54751e0efe25', '0e7cea03-2f5d-540c-b19f-ad364f00818f', NULL),
  ('79540cf9-b61e-5253-812c-8614023bf633', 'applies', 'ac133b26-b06b-567b-9f0f-54751e0efe25', '5f248184-0888-5eaa-b635-8baca4db3e03', NULL),
  ('9a6bc93c-c52e-5c72-887b-7848d680c1e0', 'applies', 'ac133b26-b06b-567b-9f0f-54751e0efe25', '69325e1b-bd93-5a40-aae0-31fcf349364e', NULL),
  ('c065fc25-af9f-51eb-b838-cc839e020cae', 'applies', 'c398925a-59af-52e7-9be4-b9eb7812ff56', '8fc58963-4c85-53b2-8dbc-8a0dc9c65909', NULL),
  ('90f38af9-2ef6-5978-8399-ac7667e726f1', 'applies', 'c398925a-59af-52e7-9be4-b9eb7812ff56', '0cb8cd99-2f95-5dc7-8737-ac8c3a2cfc20', NULL),
  ('d28e1e88-a065-5eaf-b6ff-8ba70073d1f1', 'applies', 'c398925a-59af-52e7-9be4-b9eb7812ff56', 'ba9a4de3-0d31-5545-b467-0391999a3e2c', NULL),
  ('b402de0e-379c-501f-bf7f-e8857677dde3', 'applies', '7d7512c9-85c0-5b48-89fb-263423afb7f5', '27469198-dd90-5bd0-9f2f-b286747da79d', NULL),
  ('e4d6a776-d790-5010-9f4d-c7ffd8472373', 'applies', '7d7512c9-85c0-5b48-89fb-263423afb7f5', '5b137f0b-5831-5eed-91fe-5521a3b26ec1', NULL),
  ('2dd5f493-e7d7-5666-981f-58376d05d21e', 'applies', '7d7512c9-85c0-5b48-89fb-263423afb7f5', '2d0ee8a9-553b-53e6-bca8-e36f6627fb74', NULL),
  ('2ed2429f-5ef2-5ec9-908e-8b41eedcafcb', 'applies', '7d7512c9-85c0-5b48-89fb-263423afb7f5', '5f248184-0888-5eaa-b635-8baca4db3e03', NULL),
  ('bfce5564-a254-5d00-b3e6-44b1731d3922', 'applies', 'deec0ac4-05ac-50fb-8c5d-14eee1ae3ce4', '7659fb15-aa77-5baf-97db-2e55360f2235', NULL),
  ('bd7eeaf9-4433-5169-9e8d-53b503ed53f3', 'applies', 'deec0ac4-05ac-50fb-8c5d-14eee1ae3ce4', '54c302a5-2b00-5e55-a386-09b116110c8d', NULL),
  ('96504109-f4fa-5b30-8666-2afdd611ea0f', 'applies', 'deec0ac4-05ac-50fb-8c5d-14eee1ae3ce4', 'a0405b41-0709-5763-a8c9-294c348fd6a2', NULL),
  ('3aa87e01-f591-523d-b7d5-d52dd0ad3979', 'requires', 'deec0ac4-05ac-50fb-8c5d-14eee1ae3ce4', 'e63e430c-3569-5916-8ad5-850a4380a79d', 'accurate'),
  ('7f0bb312-5b7e-5479-bc6d-943f1efcbd88', 'requires', 'ac133b26-b06b-567b-9f0f-54751e0efe25', 'deec0ac4-05ac-50fb-8c5d-14eee1ae3ce4', 'accurate'),
  ('dcc4811c-8905-5a0c-8538-5fe632a872ed', 'requires', 'c398925a-59af-52e7-9be4-b9eb7812ff56', 'ac133b26-b06b-567b-9f0f-54751e0efe25', 'accurate'),
  ('a91df247-e12f-5c13-80d5-a5399144af88', 'requires', 'c398925a-59af-52e7-9be4-b9eb7812ff56', '47e46616-8511-513a-8753-600699077de0', 'accurate'),
  ('b4a95453-7e77-5729-9754-4eea470daa2b', 'requires', '7d7512c9-85c0-5b48-89fb-263423afb7f5', 'ac133b26-b06b-567b-9f0f-54751e0efe25', 'accurate'),
  ('c1a92433-adc1-59b6-8c2b-5e1e7a8e9d48', 'requires', '31e6d82d-2724-5917-af32-78b5520bf921', 'deec0ac4-05ac-50fb-8c5d-14eee1ae3ce4', 'accurate'),
  ('0d8e854b-9ee8-5c39-a906-6fb47b5af089', 'requires', '0f48a876-b417-58ee-963f-0859b53a9301', 'ac133b26-b06b-567b-9f0f-54751e0efe25', 'accurate'),
  ('6b055446-bb1a-5eab-af9c-3354a4739ec5', 'requires', '0f48a876-b417-58ee-963f-0859b53a9301', '31e6d82d-2724-5917-af32-78b5520bf921', 'accurate'),
  ('7f1821d1-45b6-5ee8-85af-f9f49597e821', 'requires', '444f83f1-e5a9-5a2a-8f9f-512f88b08a33', 'ac133b26-b06b-567b-9f0f-54751e0efe25', 'accurate'),
  ('4b0a546d-48b2-533e-a8ac-a1990b46c0a3', 'requires', '444f83f1-e5a9-5a2a-8f9f-512f88b08a33', '31e6d82d-2724-5917-af32-78b5520bf921', 'accurate'),
  ('844b6b84-ddd2-5c04-b3d7-6da29498354a', 'requires', '4c787fe9-23bb-55b3-944b-f17266620b82', '7d7512c9-85c0-5b48-89fb-263423afb7f5', 'accurate'),
  ('1c553954-c42c-50ba-98d6-9e041aa1892b', 'requires', 'f69e7910-aff2-5e52-ae2c-544d4d74687c', '7d7512c9-85c0-5b48-89fb-263423afb7f5', 'accurate'),
  ('79f63aa5-5ee8-5b5b-bc45-e94b374ee494', 'requires', 'e92c4d0d-3d96-58be-a2c8-a377a0ce92f7', 'c398925a-59af-52e7-9be4-b9eb7812ff56', 'accurate'),
  ('c9fecc29-8ab7-5c13-b9df-4cda56350a98', 'requires', '43ed8276-0c05-5e57-babf-d5921f8d49b4', 'c398925a-59af-52e7-9be4-b9eb7812ff56', 'accurate'),
  ('818bb93f-d3fe-5a56-925b-e0f033fe3a44', 'requires', '27f09cbb-6db2-53d1-a90f-ad679d7f162f', 'c398925a-59af-52e7-9be4-b9eb7812ff56', 'accurate');
