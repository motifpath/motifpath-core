-- create "progressions" table
CREATE TABLE "progressions" ("id" uuid NOT NULL, "key" character varying NOT NULL, "mode" character varying NOT NULL, "bars" jsonb NOT NULL, "node_ids" jsonb NOT NULL, "confusable" jsonb NOT NULL, "names" jsonb NOT NULL, PRIMARY KEY ("id"));
-- create index "progressions_key_key" to table: "progressions"
CREATE UNIQUE INDEX "progressions_key_key" ON "progressions" ("key");
