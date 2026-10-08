-- modify "song_charts" table
ALTER TABLE "song_charts" ADD COLUMN "published_artist" character varying NULL, ADD COLUMN "published_concert_key" character varying NULL;
-- backfill the published artist and key of charts already published, from their published revision
UPDATE "song_charts" AS c SET "published_artist" = r."artist", "published_concert_key" = r."concert_key" FROM "song_chart_revisions" AS r WHERE r."song_chart_id" = c."id" AND r."revision_number" = c."published_revision_number";
