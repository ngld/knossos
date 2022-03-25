-- name: GetModReleaseVersions :many
SELECT m.modid, m.aid, r.version, r.updated FROM mods AS m LEFT JOIN mod_releases AS r ON r.mod_aid = m.aid;

-- name: UpdateReleaseForImport :exec
UPDATE mod_releases SET stability = pggen.arg('stability'), "description" = pggen.arg('description'),
        release_thread = pggen.arg('release_thread'), screenshots = pggen.arg('screenshots'),
        videos = pggen.arg('videos'), released = pggen.arg('released'), updated = pggen.arg('updated'),
        notes = pggen.arg('notes'), cmdline = pggen.arg('cmdline'), "private" = pggen.arg('private'),
        mod_order = pggen.arg('mod_order'),
            -- workaround since pggen forces us to pass int32 which can't be null
        teaser = CASE WHEN pggen.arg('teaser') = 0 THEN null
                 ELSE pggen.arg('teaser')
            END,
        banner = CASE WHEN pggen.arg('banner') = 0 THEN null
                 ELSE pggen.arg('banner')
            END
    WHERE mod_aid = pggen.arg('mod_aid') AND version = pggen.arg('version');
