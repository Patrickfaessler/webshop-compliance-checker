-- 0002: cooldown per start address instead of per host (C-RL); several shops can share one host.
ALTER TABLE scans ADD COLUMN target_key text NOT NULL DEFAULT '';
UPDATE scans SET target_key = start_url WHERE target_key = '';
CREATE INDEX scans_target_created_idx ON scans (target_key, created_at DESC);
