-- modify "positions" table
ALTER TABLE "positions" ADD COLUMN "custom_label" jsonb NULL, ADD COLUMN "note" jsonb NULL;
-- create "diagram_regions" table
CREATE TABLE "diagram_regions" ("id" uuid NOT NULL, "ordinal" bigint NOT NULL, "fret_start" bigint NULL, "fret_end" bigint NULL, "string_start" bigint NULL, "string_end" bigint NULL, "key_start" character varying NULL, "key_end" character varying NULL, "description" jsonb NOT NULL, "color" character varying NULL, "diagram_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "diagram_regions_diagrams_regions" FOREIGN KEY ("diagram_id") REFERENCES "diagrams" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "diagramregion_diagram_id_ordinal" to table: "diagram_regions"
CREATE UNIQUE INDEX "diagramregion_diagram_id_ordinal" ON "diagram_regions" ("diagram_id", "ordinal");
