package repository

const userColumns = `
    user_id, first_name, email, phone_number,
    password_hash, created_at
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
