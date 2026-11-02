package repository

const userColumns = `
    user_id, first_name, email, phone_number,
    password_hash, email_verified, COALESCE(pending_email, ''), created_at
`

const createUserQuery = `
    INSERT INTO user_account (
        email, password_hash, first_name, phone_number
    )
    VALUES ($1, $2, $3, $4)
    RETURNING ` + userColumns

const getUserByIDQuery = `
    SELECT ` + userColumns + `
    FROM user_account
    WHERE user_id = $1
`

const getUserByEmailQuery = `
    SELECT ` + userColumns + `
    FROM user_account
    WHERE email = $1
`

const getUserByPhoneQuery = `
    SELECT ` + userColumns + `
    FROM user_account
    WHERE phone_number = $1
`

const getProfileQuery = `
    SELECT u.user_id, u.first_name, u.email, u.phone_number, u.created_at,
           u.email_verified, COALESCE(u.pending_email, ''), u.phone_verified, c.city_id, c.name, u.about, COALESCE(u.avatar, ''),
           COALESCE(r.rating, 0), COALESCE(r.reviews_count, 0)
    FROM user_account u
    LEFT JOIN city c ON c.city_id = u.city_id
    LEFT JOIN LATERAL (
        SELECT ROUND(AVG(rv.rating), 2)::float8 AS rating, COUNT(*) AS reviews_count
        FROM review rv
        WHERE rv.seller_id = u.user_id
    ) r ON true
    WHERE u.user_id = $1
`

const updateProfileQuery = `
    UPDATE user_account
    SET first_name     = COALESCE($2, first_name),
        phone_verified = phone_verified AND COALESCE($3, phone_number) = phone_number,
        phone_number   = COALESCE($3, phone_number),
        email_verified = email_verified AND COALESCE($4, email) = email,
        email          = COALESCE($4, email),
        city_id        = COALESCE($5, city_id),
        about          = COALESCE($6, about)
    WHERE user_id = $1
`

const setEmailVerifiedQuery = `
    UPDATE user_account
    SET email_verified = true
    WHERE user_id = $1
`

const setPendingEmailQuery = `
    UPDATE user_account
    SET pending_email = $2
    WHERE user_id = $1
`

const confirmPendingEmailQuery = `
    UPDATE user_account
    SET email          = pending_email,
        pending_email  = NULL,
        email_verified = true
    WHERE user_id = $1 AND pending_email IS NOT NULL
`

const lockUserQuery = `
    SELECT 1
    FROM user_account
    WHERE user_id = $1
    FOR UPDATE
`

const getPasswordHashForUpdateQuery = `
    SELECT password_hash
    FROM user_account
    WHERE user_id = $1
    FOR UPDATE
`

const updatePasswordQuery = `
    UPDATE user_account
    SET password_hash = $2
    WHERE user_id = $1
`

const getAvatarForUpdateQuery = `
    SELECT COALESCE(avatar, '')
    FROM user_account
    WHERE user_id = $1
    FOR UPDATE
`

const updateAvatarQuery = `
    UPDATE user_account
    SET avatar = $2
    WHERE user_id = $1
`
