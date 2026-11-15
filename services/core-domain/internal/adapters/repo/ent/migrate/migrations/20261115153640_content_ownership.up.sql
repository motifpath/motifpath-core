-- create "content_ownerships" table
CREATE TABLE "content_ownerships" ("id" uuid NOT NULL, "content_kind" character varying NOT NULL, "content_id" uuid NOT NULL, "revision" bigint NOT NULL, "owner_kind" character varying NOT NULL, "owner_id" uuid NULL, "creator_id" uuid NULL, "source" character varying NOT NULL, "evidence_entry_id" uuid NOT NULL, "copied_from" jsonb NULL, "recorded_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "content_ownerships_evidence_entry_id_key" to table: "content_ownerships"
CREATE UNIQUE INDEX "content_ownerships_evidence_entry_id_key" ON "content_ownerships" ("evidence_entry_id");
-- create index "contentownership_content_kind_content_id_revision" to table: "content_ownerships"
CREATE UNIQUE INDEX "contentownership_content_kind_content_id_revision" ON "content_ownerships" ("content_kind", "content_id", "revision");
