package repository

const checkOnlineConfSchemaQuery = `
    SELECT user_id, granted_by, granted_at FROM onlineconf_admin WHERE false
`

const isOnlineConfAdminQuery = `
    SELECT EXISTS (SELECT 1 FROM onlineconf_admin WHERE user_id = $1)
`

const grantOnlineConfAdminQuery = `
    INSERT INTO onlineconf_admin (user_id, granted_by)
    VALUES ($1, $2)
    ON CONFLICT (user_id) DO NOTHING
`
