-- modify "diagrams" table: one name per language instead of a single name
ALTER TABLE "diagrams" ADD COLUMN "names" jsonb NULL;
-- each existing name becomes the English one, and no other language is invented for it
UPDATE "diagrams" SET "names" = jsonb_build_object('en', "name");
-- every diagram now has its names
ALTER TABLE "diagrams" ALTER COLUMN "names" SET NOT NULL, DROP COLUMN "name";
