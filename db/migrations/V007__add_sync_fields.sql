ALTER TABLE mod_releases DROP COLUMN IF EXISTS created;
ALTER TABLE mod_releases DROP COLUMN IF EXISTS deleted;
ALTER TABLE mods DROP COLUMN IF EXISTS tags;
ALTER TABLE mods DROP COLUMN IF EXISTS parent;
ALTER TABLE mod_packages DROP COLUMN IF EXISTS knossos_vp;
ALTER TABLE mod_package_files DROP COLUMN IF EXISTS archive_id;

ALTER TABLE mod_releases ADD COLUMN created timestamp with time zone DEFAULT NOW();
ALTER TABLE mod_releases ADD COLUMN deleted boolean NOT NULL DEFAULT false;
ALTER TABLE mods ADD COLUMN tags text[] NOT NULL DEFAULT '{}';
ALTER TABLE mods ADD COLUMN parent text NOT NULL DEFAULT 'FS2';
ALTER TABLE mod_packages ADD COLUMN knossos_vp boolean NOT NULL DEFAULT false;

ALTER TABLE mod_package_files ADD COLUMN archive_id integer;
UPDATE mod_package_files AS pf SET archive_id = (SELECT id FROM mod_package_archives WHERE package_id = pf.package_id AND label = pf.archive);
ALTER TABLE mod_package_files ALTER COLUMN archive_id SET NOT NULL;
ALTER TABLE mod_package_files ADD FOREIGN KEY (archive_id) REFERENCES mod_package_archives (id);
ALTER TABLE mod_package_files DROP COLUMN archive;
