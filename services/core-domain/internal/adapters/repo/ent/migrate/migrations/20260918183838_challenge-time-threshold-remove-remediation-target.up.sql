-- modify "challenges" table
ALTER TABLE "challenges" DROP COLUMN "remediation_target_content_node_id", ADD COLUMN "time_threshold_ms" bigint NULL;
