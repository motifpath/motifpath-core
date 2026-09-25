-- modify "course_versions" table
ALTER TABLE "course_versions" ADD COLUMN "language_snapshot" character varying NOT NULL DEFAULT 'en';
-- modify "courses" table
ALTER TABLE "courses" ADD COLUMN "language" character varying NOT NULL DEFAULT 'en';
