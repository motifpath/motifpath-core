-- modify "users" table
ALTER TABLE "users" ADD COLUMN "email" character varying NULL, ADD COLUMN "time_zone" character varying NULL, ADD COLUMN "email_progress_report" boolean NOT NULL DEFAULT true, ADD COLUMN "email_reminders" boolean NOT NULL DEFAULT true;
