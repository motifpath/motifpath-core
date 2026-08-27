-- create "challenges" table
CREATE TABLE "challenges" ("id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "subject_tag" character varying NOT NULL, "pass_threshold" bigint NOT NULL, "remediation_target_content_node_id" uuid NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "challenge_content_node_id" to table: "challenges"
CREATE INDEX "challenge_content_node_id" ON "challenges" ("content_node_id");
-- create "content_nodes" table
CREATE TABLE "content_nodes" ("id" uuid NOT NULL, "teacher_id" uuid NOT NULL, "title" character varying NOT NULL, "content_type" character varying NOT NULL, "skill" character varying NOT NULL, "concept" character varying NOT NULL, "difficulty_level" character varying NOT NULL, "review_state" character varying NOT NULL DEFAULT 'pending', "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "exercises" table
CREATE TABLE "exercises" ("id" uuid NOT NULL, "challenge_id" uuid NOT NULL, "exercise_type" character varying NOT NULL, "prompt" text NOT NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "exercise_challenge_id" to table: "exercises"
CREATE INDEX "exercise_challenge_id" ON "exercises" ("challenge_id");
-- create "expanded_contents" table
CREATE TABLE "expanded_contents" ("id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "content_type" character varying NOT NULL, "media_url" character varying NOT NULL, "trigger_at_seconds" bigint NULL, "hide_at_seconds" bigint NULL, "trigger_at_paragraph" bigint NULL, "duration_ms" bigint NULL, "caption" character varying NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "expandedcontent_content_node_id" to table: "expanded_contents"
CREATE INDEX "expandedcontent_content_node_id" ON "expanded_contents" ("content_node_id");
-- create "learning_paths" table
CREATE TABLE "learning_paths" ("id" uuid NOT NULL, "teacher_id" uuid NOT NULL, "title" character varying NOT NULL, "created_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create "learning_path_items" table
CREATE TABLE "learning_path_items" ("id" uuid NOT NULL, "learning_path_id" uuid NOT NULL, "content_node_id" uuid NOT NULL, "position" bigint NOT NULL, PRIMARY KEY ("id"));
-- create index "learningpathitem_learning_path_id_position" to table: "learning_path_items"
CREATE UNIQUE INDEX "learningpathitem_learning_path_id_position" ON "learning_path_items" ("learning_path_id", "position");
-- create "path_assignments" table
CREATE TABLE "path_assignments" ("id" uuid NOT NULL, "student_id" uuid NOT NULL, "learning_path_id" uuid NOT NULL, "assigned_by" uuid NOT NULL, "assigned_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "path_assignments_student_id_key" to table: "path_assignments"
CREATE UNIQUE INDEX "path_assignments_student_id_key" ON "path_assignments" ("student_id");
-- create "users" table
CREATE TABLE "users" ("id" uuid NOT NULL, "clerk_user_id" character varying NOT NULL, "role" character varying NOT NULL, "registered_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "users_clerk_user_id_key" to table: "users"
CREATE UNIQUE INDEX "users_clerk_user_id_key" ON "users" ("clerk_user_id");
