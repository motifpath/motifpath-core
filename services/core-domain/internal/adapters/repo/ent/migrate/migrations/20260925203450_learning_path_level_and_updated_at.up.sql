-- modify "learning_paths" table: an authored level, and when the path last changed
ALTER TABLE "learning_paths" ADD COLUMN "level" character varying NULL, ADD COLUMN "updated_at" timestamptz NULL;
-- existing paths were last changed no later than they were created, as far as anything recorded
UPDATE "learning_paths" SET "updated_at" = "created_at";
-- every path now has its last update
ALTER TABLE "learning_paths" ALTER COLUMN "updated_at" SET NOT NULL;
