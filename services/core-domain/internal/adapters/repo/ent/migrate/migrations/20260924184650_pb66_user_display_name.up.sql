-- modify "users" table: users that predate display names get a placeholder,
-- which their real name replaces on their next authenticated request
ALTER TABLE "users" ADD COLUMN "display_name" character varying NOT NULL DEFAULT 'MotifPath user';
-- the column keeps no default: every new user is created with their name
ALTER TABLE "users" ALTER COLUMN "display_name" DROP DEFAULT;
