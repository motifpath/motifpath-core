-- modify "exercises" table
ALTER TABLE "exercises" ADD COLUMN "remediation_targets" text NULL;
-- modify "expanded_contents" table
ALTER TABLE "expanded_contents" ALTER COLUMN "media_url" DROP NOT NULL, ADD COLUMN "rich_content" text NULL;
