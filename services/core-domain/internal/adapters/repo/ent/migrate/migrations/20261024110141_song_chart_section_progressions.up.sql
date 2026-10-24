-- modify "song_chart_revisions" table
ALTER TABLE "song_chart_revisions" DROP COLUMN "progression_keys", ADD COLUMN "section_progressions" jsonb NOT NULL DEFAULT '[]';
-- modify "song_charts" table
ALTER TABLE "song_charts" DROP COLUMN "progression_keys", DROP COLUMN "unplayed_progression_keys", ADD COLUMN "unplayed_progressions" jsonb NOT NULL DEFAULT '[]';
