-- create "app_configs" table
CREATE TABLE "app_configs" ("key" character varying NOT NULL, "value" jsonb NOT NULL, "updated_at" timestamptz NOT NULL, "updated_by" uuid NOT NULL, PRIMARY KEY ("key"));
