-- modify "progressions" table
ALTER TABLE "progressions" ADD COLUMN "in_drills" boolean NOT NULL DEFAULT false, ADD COLUMN "nicknames" jsonb NULL, ADD COLUMN "created_by" uuid NULL, ADD COLUMN "created_at" timestamptz NULL;
-- create index "progression_mode_bars" to table: "progressions"
CREATE UNIQUE INDEX "progression_mode_bars" ON "progressions" ("mode", "bars");
-- every progression so far is the catalog's, and the catalog is the drills'
UPDATE "progressions" SET "in_drills" = true;
