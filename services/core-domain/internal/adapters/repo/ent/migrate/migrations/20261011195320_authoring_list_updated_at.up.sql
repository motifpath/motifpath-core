-- Every row was last changed when it was created until it is next edited.
-- modify "content_nodes" table
ALTER TABLE "content_nodes" ADD COLUMN "updated_at" timestamptz NULL, ADD COLUMN "latest_published_version" bigint NULL, ADD COLUMN "has_unpublished_changes" boolean NOT NULL DEFAULT true;
UPDATE "content_nodes" SET "updated_at" = "created_at";
-- A node with versions takes its latest and is assumed to match it.
UPDATE "content_nodes" AS n SET "latest_published_version" = v.latest, "has_unpublished_changes" = false
FROM (SELECT "content_node_id", MAX("version_number") AS latest FROM "content_node_versions" GROUP BY "content_node_id") AS v
WHERE v."content_node_id" = n."id";
ALTER TABLE "content_nodes" ALTER COLUMN "updated_at" SET NOT NULL;
-- modify "courses" table
ALTER TABLE "courses" ADD COLUMN "updated_at" timestamptz NULL;
UPDATE "courses" SET "updated_at" = "created_at";
ALTER TABLE "courses" ALTER COLUMN "updated_at" SET NOT NULL;
-- modify "diagrams" table
ALTER TABLE "diagrams" ADD COLUMN "updated_at" timestamptz NULL;
UPDATE "diagrams" SET "updated_at" = "created_at";
ALTER TABLE "diagrams" ALTER COLUMN "updated_at" SET NOT NULL;
-- modify "exercises" table
ALTER TABLE "exercises" ADD COLUMN "updated_at" timestamptz NULL;
UPDATE "exercises" SET "updated_at" = "created_at";
ALTER TABLE "exercises" ALTER COLUMN "updated_at" SET NOT NULL;
