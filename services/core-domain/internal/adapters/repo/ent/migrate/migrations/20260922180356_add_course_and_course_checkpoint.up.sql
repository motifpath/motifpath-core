-- create "courses" table
CREATE TABLE "courses" ("id" uuid NOT NULL, "title" character varying NOT NULL, "summary" character varying NOT NULL, "level" character varying NOT NULL, "status" character varying NOT NULL DEFAULT 'draft', "created_by" uuid NOT NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "course_checkpoints" table
CREATE TABLE "course_checkpoints" ("id" uuid NOT NULL, "course_id" uuid NOT NULL, "learning_path_id" uuid NOT NULL, "position" bigint NOT NULL, "title" character varying NULL, PRIMARY KEY ("id"));
-- create index "coursecheckpoint_course_id_position" to table: "course_checkpoints"
CREATE UNIQUE INDEX "coursecheckpoint_course_id_position" ON "course_checkpoints" ("course_id", "position");
