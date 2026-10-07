-- create "diagram_shape_families" table
CREATE TABLE "diagram_shape_families" ("id" uuid NOT NULL, "key" character varying NOT NULL, "names" jsonb NOT NULL, "members" jsonb NOT NULL, PRIMARY KEY ("id"));
-- create index "diagram_shape_families_key_key" to table: "diagram_shape_families"
CREATE UNIQUE INDEX "diagram_shape_families_key_key" ON "diagram_shape_families" ("key");
-- create "diagram_shapes" table
CREATE TABLE "diagram_shapes" ("id" uuid NOT NULL, "shape" character varying NOT NULL, "diagram_id" uuid NOT NULL, "family_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "diagram_shapes_diagram_shape_families_family" FOREIGN KEY ("family_id") REFERENCES "diagram_shape_families" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, CONSTRAINT "diagram_shapes_diagrams_shape" FOREIGN KEY ("diagram_id") REFERENCES "diagrams" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "diagram_shapes_diagram_id_key" to table: "diagram_shapes"
CREATE UNIQUE INDEX "diagram_shapes_diagram_id_key" ON "diagram_shapes" ("diagram_id");
