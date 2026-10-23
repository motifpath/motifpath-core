-- modify "song_chart_revisions" table
ALTER TABLE "song_chart_revisions" ADD COLUMN "progression_keys" jsonb NOT NULL DEFAULT '[]';
-- modify "song_charts" table
ALTER TABLE "song_charts" ADD COLUMN "progression_keys" jsonb NOT NULL DEFAULT '[]', ADD COLUMN "unplayed_progression_keys" jsonb NOT NULL DEFAULT '[]';
