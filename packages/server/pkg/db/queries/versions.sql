-- name: GetVersions :many
SELECT key, version FROM versions;

-- name: UpdateVersion :exec
INSERT INTO versions (key, version) VALUES (pggen.arg('key'), pggen.arg('version'))
	ON CONFLICT (key) DO UPDATE SET key = pggen.arg('key'), version = pggen.arg('version');
