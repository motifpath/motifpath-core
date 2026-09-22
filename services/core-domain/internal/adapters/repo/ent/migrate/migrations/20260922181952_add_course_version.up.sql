-- create "course_versions" table
CREATE TABLE "course_versions" ("id" uuid NOT NULL, "course_id" uuid NOT NULL, "version_number" bigint NOT NULL, "title_snapshot" character varying NOT NULL, "summary_snapshot" character varying NOT NULL, "level_snapshot" character varying NOT NULL, "available_for_new_enrollments" boolean NOT NULL DEFAULT true, "published_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "courseversion_course_id_version_number" to table: "course_versions"
CREATE UNIQUE INDEX "courseversion_course_id_version_number" ON "course_versions" ("course_id", "version_number");
-- create "course_version_checkpoints" table
CREATE TABLE "course_version_checkpoints" ("id" uuid NOT NULL, "course_version_id" uuid NOT NULL, "learning_path_id" uuid NOT NULL, "position" bigint NOT NULL, "effective_title" character varying NOT NULL, PRIMARY KEY ("id"));
-- create index "courseversioncheckpoint_course_version_id_position" to table: "course_version_checkpoints"
CREATE UNIQUE INDEX "courseversioncheckpoint_course_version_id_position" ON "course_version_checkpoints" ("course_version_id", "position");
