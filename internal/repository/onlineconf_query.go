package repository

const createConf = `
		INSERT INTO onlineconf_entries (scope, name, kind, value)
		VALUES ($1, $2, $3, $4::jsonb)
		RETURNING id, scope, name, kind, value`

const updateConf = `
		UPDATE onlineconf_entries
		SET scope = COALESCE($2, scope),
		    name  = COALESCE($3, name),
		    value = COALESCE($4::jsonb, value)
		WHERE id = $1
		RETURNING id, scope, name, kind, value`

const searchConf = `
		SELECT id, scope, name, kind, value
		FROM onlineconf_entries
		WHERE strpos(lower(name), lower($1)) > 0
		  AND ($2::text IS NULL OR scope = $2)
		  AND ($3::text IS NULL OR kind = $3)
		ORDER BY id`
