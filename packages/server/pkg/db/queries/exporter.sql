-- name: GetPublicModUpdatedDates :many
SELECT m.modid, m.aid, m.title, m.tags, MAX(r.updated) AS updated FROM mods AS m
    LEFT JOIN mod_releases AS r ON r.mod_aid = m.aid
    GROUP BY m.modid, m.aid;

-- name: GetPublicModReleasesByAid :many
SELECT r.*, m.modid FROM mod_releases AS r
    INNER JOIN mods AS m ON m.aid = r.mod_aid
    WHERE mod_aid = pggen.arg('aid') ORDER BY created ASC;

-- name: GetPublicModReleasesByAIDSince :many
SELECT r.*, m.modid FROM mod_releases AS r
    INNER JOIN mods AS m ON m.aid = r.mod_aid
    WHERE mod_aid = pggen.arg('aid') AND updated > pggen.arg('since')
    ORDER BY created ASC;

-- name: GetPublicPackagesByReleaseID :many
SELECT p.* FROM mod_packages AS p
    INNER JOIN mod_releases AS r ON r.id = p.release_id
    LEFT JOIN mods AS m ON m.aid = r.mod_aid
    WHERE m.private = false AND r.id = pggen.arg('id');

-- name: GetPublicPackageDependencsByReleaseID :many
SELECT pd.* FROM mod_package_dependencies AS pd
    INNER JOIN mod_packages AS p ON p.id = pd.package_id
    INNER JOIN mod_releases AS r ON r.id = p.release_id
    LEFT JOIN mods AS m ON m.aid = r.mod_aid
    WHERE r.id = pggen.arg('id') AND m.private = false;

-- name: GetPublicPackageArchivesByReleaseID :many
SELECT f.filesize, f.storage_key, f.external, pa.* FROM mod_package_archives AS pa
    LEFT JOIN files AS f ON f.id = pa.file_id
    INNER JOIN mod_packages AS p ON p.id = pa.package_id
    INNER JOIN mod_releases AS r ON r.id = p.release_id
    LEFT JOIN mods AS m ON m.aid = r.mod_aid
    WHERE m.private = false AND r.id = pggen.arg('id');

-- name: GetPublicPackageFilesByReleaseID :many
SELECT pf.* FROM mod_package_files AS pf
    INNER JOIN mod_packages AS p ON p.id = pf.package_id
    INNER JOIN mod_releases AS r ON r.id = p.release_id
    LEFT JOIN mods AS m ON m.aid = r.mod_aid
    WHERE m.private = false AND r.id = pggen.arg('id');

-- name: GetPublicPackageExecutablesByReleaseID :many
SELECT pe.* FROM mod_package_executables AS pe
    INNER JOIN mod_packages AS p ON p.id = pe.package_id
    INNER JOIN mod_releases AS r ON r.id = p.release_id
    LEFT JOIN mods AS m ON m.aid = r.mod_aid
    WHERE m.private = false AND r.id = pggen.arg('id');

-- name: GetChecksumsByReleaseID :many
SELECT jsonb_object_agg(pf.path, pf.checksum_digest) AS files,
        pa.id, pa.label, pa.checksum_digest, f.id AS fid, f.filesize, f.storage_key, f.external
    FROM mod_package_files AS pf
    INNER JOIN mod_package_archives AS pa ON pa.id = pf.archive_id
    LEFT JOIN files AS f ON f.id = pa.file_id
    INNER JOIN mod_packages AS p ON p.id = pf.package_id
    INNER JOIN mod_releases AS r ON r.id = p.release_id
    WHERE r.id = pggen.arg('id')
    GROUP BY pa.id, f.id;
