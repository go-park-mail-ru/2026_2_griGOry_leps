package repository

const createPasswordResetQuery = `
    INSERT INTO password_reset (token_hash, user_id, expires_at)
    VALUES ($1, $2, $3)
`

const hasPasswordResetSinceQuery = `
    SELECT EXISTS (
        SELECT 1
        FROM password_reset
        WHERE user_id = $1 AND created_at > $2
    )
`

const getPasswordResetQuery = `
    SELECT user_id, expires_at, used_at
    FROM password_reset
    WHERE token_hash = $1
`

const getPasswordResetForUpdateQuery = getPasswordResetQuery + `    FOR UPDATE
`

const usePasswordResetsQuery = `
    UPDATE password_reset
    SET used_at = now()
    WHERE user_id = $1 AND used_at IS NULL
`
