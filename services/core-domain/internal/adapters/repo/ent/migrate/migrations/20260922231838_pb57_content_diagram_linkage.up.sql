-- modify "exercise_options" table
ALTER TABLE "exercise_options" ADD COLUMN "diagram_ref" text NULL, ADD COLUMN "diagram_id" uuid NULL, ADD COLUMN "diagram_position_id" uuid NULL;
-- modify "exercises" table
ALTER TABLE "exercises" ADD COLUMN "diagram_ref" text NULL, ADD COLUMN "diagram_stack_ref" text NULL;
-- modify "expanded_contents" table
ALTER TABLE "expanded_contents" ADD COLUMN "diagram_ref" text NULL, ADD COLUMN "diagram_stack_ref" text NULL;
