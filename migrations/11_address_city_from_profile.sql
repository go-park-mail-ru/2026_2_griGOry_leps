-- +goose Up
UPDATE address a
SET city = c.name
FROM user_account u
JOIN city c ON c.city_id = u.city_id
WHERE u.user_id = a.user_id AND a.city = '';

-- +goose Down
