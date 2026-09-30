-- modify "learning_paths" table
ALTER TABLE "learning_paths" ADD COLUMN "summary" character varying NULL, ADD COLUMN "language" character varying NULL, ADD COLUMN "status" character varying NOT NULL DEFAULT 'draft';
-- modify "student_paths" table
ALTER TABLE "student_paths" ADD COLUMN "summary_snapshot" character varying NULL, ADD COLUMN "level_snapshot" character varying NULL, ADD COLUMN "thumbnail_url_snapshot" character varying NULL, ADD COLUMN "created_by_snapshot" uuid NULL;
-- create index "studentpath_student_id_source_template_id" to table: "student_paths"
CREATE UNIQUE INDEX "studentpath_student_id_source_template_id" ON "student_paths" ("student_id", "source_template_id") WHERE ((archived_at IS NULL) AND (source_course_enrollment_id IS NULL));
