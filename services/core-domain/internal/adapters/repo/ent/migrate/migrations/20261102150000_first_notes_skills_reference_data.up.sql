-- Frozen knowledge map, compiled from motifpath-specs catalogs/knowledge-map.yaml by
-- scripts/knowledge_map. Each id is the UUID v5 of knowledge-node/<key> or
-- knowledge-edge/<type>/<from key>/<to key>. A key or id already in use stops the migration.
INSERT INTO "knowledge_nodes" ("id", "kind", "key", "names", "parent_id") VALUES
  ('b172ca6a-311a-5ad6-ae5c-f249e7ecd55c', 'skill', 'interpret-note-names', '{"en":"Interpret note names","pt_BR":"Interpretar os nomes das notas"}', '3a51c665-a147-5e56-b232-fff2a4b073be'),
  ('e63e430c-3569-5916-8ad5-850a4380a79d', 'skill', 'count-semitone-steps', '{"en":"Count semitone steps from a visual reference","pt_BR":"Contar semitons com uma referência visual"}', '3a51c665-a147-5e56-b232-fff2a4b073be');
INSERT INTO "knowledge_edges" ("id", "type", "from_id", "to_id", "level") VALUES
  ('1496d8b4-abd8-51ad-84bf-c5008e6ea085', 'applies', 'b172ca6a-311a-5ad6-ae5c-f249e7ecd55c', 'fd57c836-aa23-580c-92ed-aca64850393b', NULL),
  ('4674824b-a2e6-538e-b5bf-93776f5b8bc4', 'applies', 'b172ca6a-311a-5ad6-ae5c-f249e7ecd55c', '8b31d551-5475-5e9d-bd02-59b1f0bd2d71', NULL),
  ('c4c9ea4d-9399-5a31-90b0-1d4e4e8f8b86', 'applies', 'e63e430c-3569-5916-8ad5-850a4380a79d', 'a0405b41-0709-5763-a8c9-294c348fd6a2', NULL),
  ('6af721bb-22fb-5ed6-9c03-a859687cf6b2', 'applies', 'e63e430c-3569-5916-8ad5-850a4380a79d', '1a273528-6153-538d-905e-d28d8ff9f6fb', NULL);
