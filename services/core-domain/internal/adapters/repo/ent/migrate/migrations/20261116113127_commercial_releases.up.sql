-- create "commercial_content_states" table
CREATE TABLE "commercial_content_states" ("id" uuid NOT NULL, "content_kind" character varying NOT NULL, "content_id" uuid NOT NULL, "revision" bigint NOT NULL, "latest_release_id" uuid NULL, "updated_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "commercialcontentstate_content_kind_content_id" to table: "commercial_content_states"
CREATE UNIQUE INDEX "commercialcontentstate_content_kind_content_id" ON "commercial_content_states" ("content_kind", "content_id");
-- create "commercial_releases" table
CREATE TABLE "commercial_releases" ("id" uuid NOT NULL, "content_kind" character varying NOT NULL, "content_id" uuid NOT NULL, "revision" bigint NOT NULL, "manifest_sha256" character varying NOT NULL, "content_sha256" character varying NOT NULL, "core_version_ref" character varying NULL, "ownership" jsonb NOT NULL, "dependencies" jsonb NOT NULL, "evidence_entry_id" uuid NOT NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "commercialrelease_content_kind_content_id_revision" to table: "commercial_releases"
CREATE UNIQUE INDEX "commercialrelease_content_kind_content_id_revision" ON "commercial_releases" ("content_kind", "content_id", "revision");
