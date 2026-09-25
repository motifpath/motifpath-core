-- modify "content_node_versions" table
ALTER TABLE "content_node_versions" ADD COLUMN "thumbnail_url_snapshot" character varying NULL;
-- modify "content_nodes" table
ALTER TABLE "content_nodes" ADD COLUMN "thumbnail_url" character varying NULL;
-- modify "course_enrollments" table
ALTER TABLE "course_enrollments" ADD COLUMN "course_thumbnail_url" character varying NULL;
-- modify "course_versions" table
ALTER TABLE "course_versions" ADD COLUMN "thumbnail_url_snapshot" character varying NULL;
-- modify "courses" table
ALTER TABLE "courses" ADD COLUMN "thumbnail_url" character varying NULL;
-- modify "learning_paths" table
ALTER TABLE "learning_paths" ADD COLUMN "thumbnail_url" character varying NULL;
