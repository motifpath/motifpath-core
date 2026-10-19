-- modify "diagrams" table
ALTER TABLE "diagrams" ADD COLUMN "purpose" character varying NOT NULL DEFAULT 'general';
