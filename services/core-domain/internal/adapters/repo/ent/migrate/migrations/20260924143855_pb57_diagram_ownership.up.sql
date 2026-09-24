-- modify "diagrams" table: existing rows become basic templates
ALTER TABLE "diagrams" ADD COLUMN "kind" character varying NOT NULL DEFAULT 'basic', ADD COLUMN "created_by" uuid NULL;
-- the column keeps no default: every new diagram states its kind explicitly
ALTER TABLE "diagrams" ALTER COLUMN "kind" DROP DEFAULT;
-- existing diagrams are owned by the zero user: the earliest-registered admin
UPDATE "diagrams" SET "created_by" = (SELECT "id" FROM "users" WHERE "role" = 'admin' ORDER BY "registered_at", "id" LIMIT 1) WHERE "created_by" IS NULL;
-- refuse to invent an owner: when diagrams exist but no admin does, this
-- cast fails, and its error message is the explanation the operator sees
-- (the CASE reads a column so Postgres can't fold the cast into a
-- plan-time constant that fails even when no row matches)
SELECT CAST(CASE WHEN "created_by" IS NULL THEN 'no admin user exists to own the existing diagrams (create the admin user first, then re-run this migration)' END AS integer) FROM "diagrams" WHERE "created_by" IS NULL LIMIT 1;
-- every diagram now has an owner
ALTER TABLE "diagrams" ALTER COLUMN "created_by" SET NOT NULL;
