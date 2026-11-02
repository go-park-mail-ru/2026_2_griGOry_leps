package repository

const createEmailVerificationQuery = `
    INSERT INTO email_verification (token_hash, user_id, email, expires_at)
    VALUES ($1, $2, $3, $4)
`

const lastEmailVerificationQuery = `
    SELECT max(created_at)
    FROM email_verification
    WHERE user_id = $1
`

const getEmailVerificationQuery = `
    SELECT user_id, email, expires_at, used_at
    FROM email_verification
    WHERE token_hash = $1
`

const getEmailVerificationForUpdateQuery = getEmailVerificationQuery + `    FOR UPDATE
`

const useEmailVerificationsQuery = `
    UPDATE email_verification
    SET used_at = now()
    WHERE user_id = $1 AND used_at IS NULL
`
