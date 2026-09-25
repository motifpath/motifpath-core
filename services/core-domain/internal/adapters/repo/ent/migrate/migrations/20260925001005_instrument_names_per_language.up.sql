-- modify "instruments" table: one name per language instead of a single name
ALTER TABLE "instruments" ADD COLUMN "names" jsonb NULL;
-- each existing name becomes the English one, and no other language is invented for it
UPDATE "instruments" SET "names" = jsonb_build_object('en', "name");
-- every instrument now has its names
ALTER TABLE "instruments" ALTER COLUMN "names" SET NOT NULL, DROP COLUMN "name";
