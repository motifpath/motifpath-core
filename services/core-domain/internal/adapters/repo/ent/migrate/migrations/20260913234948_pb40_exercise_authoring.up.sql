-- modify "exercises" table
ALTER TABLE "exercises" DROP COLUMN "challenge_id", ADD COLUMN "title" character varying NOT NULL, ADD COLUMN "skill_tags" jsonb NULL, ADD COLUMN "image_url" character varying NULL, ADD COLUMN "audio_url" character varying NULL;
-- create "exercise_challenges" table
CREATE TABLE "exercise_challenges" ("exercise_id" uuid NOT NULL, "challenge_id" uuid NOT NULL, PRIMARY KEY ("exercise_id", "challenge_id"), CONSTRAINT "exercise_challenges_challenge_id" FOREIGN KEY ("challenge_id") REFERENCES "challenges" ("id") ON UPDATE NO ACTION ON DELETE CASCADE, CONSTRAINT "exercise_challenges_exercise_id" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE CASCADE);
-- create "exercise_options" table
CREATE TABLE "exercise_options" ("id" uuid NOT NULL, "is_correct" boolean NOT NULL, "label" character varying NULL, "image_url" character varying NULL, "region_x" double precision NULL, "region_y" double precision NULL, "region_width" double precision NULL, "region_height" double precision NULL, "region_shape" character varying NULL, "exercise_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "exercise_options_exercises_options" FOREIGN KEY ("exercise_id") REFERENCES "exercises" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "exerciseoption_exercise_id" to table: "exercise_options"
CREATE INDEX "exerciseoption_exercise_id" ON "exercise_options" ("exercise_id");
