package repository

const createSessionQuery = `
    INSERT INTO session (id, user_id, expires_at)
    VALUES ($1, $2, $3)
`

const getSessionQuery = `
    SELECT id, user_id, expires_at
    FROM session
    WHERE id = $1
`

const deleteSessionQuery = `
    DELETE FROM session
    WHERE id = $1
`
