-- modify "diagrams" table
ALTER TABLE "diagrams" ADD COLUMN "root_note" character varying NULL, ADD COLUMN "label_display" character varying NOT NULL DEFAULT 'interval';
-- modify "positions" table
ALTER TABLE "positions" ADD COLUMN "shape" character varying NOT NULL DEFAULT 'dot';
