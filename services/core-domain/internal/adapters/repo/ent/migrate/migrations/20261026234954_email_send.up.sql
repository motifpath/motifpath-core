-- create "email_sends" table
CREATE TABLE "email_sends" ("id" uuid NOT NULL, "recipient" character varying NOT NULL, "kind" character varying NOT NULL, "period_key" character varying NOT NULL, "sent_at" timestamptz NOT NULL, PRIMARY KEY ("id"));
-- create index "emailsend_recipient_kind_period_key" to table: "email_sends"
CREATE UNIQUE INDEX "emailsend_recipient_kind_period_key" ON "email_sends" ("recipient", "kind", "period_key");
