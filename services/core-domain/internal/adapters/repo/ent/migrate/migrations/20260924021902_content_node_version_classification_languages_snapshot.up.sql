-- modify "content_node_versions" table
ALTER TABLE "content_node_versions" ADD COLUMN "classification_snapshot" text NULL, ADD COLUMN "languages_snapshot" text NULL;
