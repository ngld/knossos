ALTER TABLE mod_releases DROP COLUMN IF EXISTS dependency_snapshot;
ALTER TABLE mod_releases DROP COLUMN IF EXISTS install_count;
ALTER TABLE mod_releases DROP COLUMN IF EXISTS mod_order;

ALTER TABLE mod_releases ADD COLUMN dependency_snapshot jsonb NOT NULL DEFAULT '{}';
ALTER TABLE mod_releases ADD COLUMN install_count int NOT NULL DEFAULT 0;
ALTER TABLE mod_releases ADD COLUMN mod_order text[] NOT NULL DEFAULT '{}';
