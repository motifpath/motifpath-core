-- create "drill_templates" table
CREATE TABLE "drill_templates" ("id" uuid NOT NULL, "key" character varying NOT NULL, "item_kind" character varying NOT NULL, "response_type" character varying NOT NULL, "timed" boolean NOT NULL, "names" jsonb NOT NULL, PRIMARY KEY ("id"));
-- create index "drill_templates_key_key" to table: "drill_templates"
CREATE UNIQUE INDEX "drill_templates_key_key" ON "drill_templates" ("key");
-- create "drill_thresholds" table
CREATE TABLE "drill_thresholds" ("id" uuid NOT NULL, "version" bigint NOT NULL, "effective_from" timestamptz NOT NULL, "fluent_net_ms" bigint NOT NULL, "source" character varying NOT NULL, "sessions" bigint NOT NULL, "students" bigint NOT NULL, "template_id" uuid NOT NULL, PRIMARY KEY ("id"), CONSTRAINT "drill_thresholds_drill_templates_template" FOREIGN KEY ("template_id") REFERENCES "drill_templates" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION);
-- create index "drillthreshold_template_id_version" to table: "drill_thresholds"
CREATE UNIQUE INDEX "drillthreshold_template_id_version" ON "drill_thresholds" ("template_id", "version");
