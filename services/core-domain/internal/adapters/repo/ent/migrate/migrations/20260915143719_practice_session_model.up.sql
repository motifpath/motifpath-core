-- modify "challenges" table
ALTER TABLE "challenges" ADD COLUMN "shuffle_exercises" boolean NOT NULL DEFAULT false, ADD COLUMN "shuffle_options" boolean NOT NULL DEFAULT false;
-- modify "exercises" table
ALTER TABLE "exercises" ADD COLUMN "estimated_duration_seconds" bigint NULL;
-- create "exercise_content_nodes" table
CREATE TABLE "exercise_content_nodes" ("exercise_id" uuid NOT NULL, "content_node_id" uuid NOT NULL, PRIMARY KEY ("exercise_id", "content_node_id"), CONSTRAINT "exercise_content_nodes_content_node_id" FOREIGN KEY ("content_node_id") REFERENCES "content_nodes" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "exercise_content_nodes_exercise_id" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
