-- create "change_targets" table
CREATE TABLE "change_targets" ("id" uuid NOT NULL, "version" bigint NOT NULL, "effective_from" timestamptz NOT NULL, "changes_per_minute" bigint NOT NULL, "source" character varying NOT NULL, PRIMARY KEY ("id"));
-- create index "change_targets_version_key" to table: "change_targets"
CREATE UNIQUE INDEX "change_targets_version_key" ON "change_targets" ("version");
-- create "chord_change_pairs" table
CREATE TABLE "chord_change_pairs" ("id" uuid NOT NULL, "from_chord_drill_id" uuid NOT NULL, "to_chord_drill_id" uuid NOT NULL, "ordinal" bigint NOT NULL, PRIMARY KEY ("id"));
-- create index "chord_change_pairs_ordinal_key" to table: "chord_change_pairs"
CREATE UNIQUE INDEX "chord_change_pairs_ordinal_key" ON "chord_change_pairs" ("ordinal");
-- create index "chordchangepair_from_chord_drill_id_to_chord_drill_id" to table: "chord_change_pairs"
CREATE UNIQUE INDEX "chordchangepair_from_chord_drill_id_to_chord_drill_id" ON "chord_change_pairs" ("from_chord_drill_id", "to_chord_drill_id");
-- create "chord_drills" table
CREATE TABLE "chord_drills" ("id" uuid NOT NULL, "diagram_id" uuid NOT NULL, "chord" character varying NOT NULL, "ordinal" bigint NOT NULL, PRIMARY KEY ("id"));
-- create index "chord_drills_diagram_id_key" to table: "chord_drills"
CREATE UNIQUE INDEX "chord_drills_diagram_id_key" ON "chord_drills" ("diagram_id");
-- create index "chord_drills_ordinal_key" to table: "chord_drills"
CREATE UNIQUE INDEX "chord_drills_ordinal_key" ON "chord_drills" ("ordinal");
