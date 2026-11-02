package repository

const cartEntryColumns = `
    ci.ad_id, COALESCE(a.title, ''), a.price, (
        SELECT url FROM ad_image
        WHERE ad_id = ci.ad_id
        ORDER BY position, image_id
        LIMIT 1
    ), a.address, COALESCE(a.has_delivery, false), COALESCE(a.status, 'deleted'),
    u.user_id, u.first_name, COALESCE(u.avatar, ''), COALESCE(stats.rating, 0)
`

const cartSellerJoin = `
    JOIN user_account u ON u.user_id = ci.seller_id
    LEFT JOIN LATERAL (
        SELECT ROUND(AVG(rv.rating), 2)::float8 AS rating
        FROM review rv
        WHERE rv.seller_id = u.user_id
    ) stats ON true
`

const getCartQuery = `
    SELECT ` + cartEntryColumns + `
    FROM cart c
    JOIN cart_item ci ON ci.cart_id = c.id
    LEFT JOIN ad a ON a.id = ci.ad_id
` + cartSellerJoin + `
    WHERE c.user_id = $1
    ORDER BY ci.id DESC
`

const cartEntriesByIDsQuery = `
    SELECT ` + cartEntryColumns + `
    FROM cart c
    JOIN cart_item ci ON ci.cart_id = c.id
    LEFT JOIN ad a ON a.id = ci.ad_id
` + cartSellerJoin + `
    WHERE c.user_id = $1 AND ci.ad_id = ANY($2)
    ORDER BY ci.id DESC
`

const getAdForCartQuery = `
    SELECT a.user_id, a.status
    FROM ad a
    WHERE a.id = $1
    FOR KEY SHARE
`

const ensureCartQuery = `
    INSERT INTO cart (user_id) VALUES ($1)
    ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
    RETURNING id
`

const addCartItemQuery = `
    INSERT INTO cart_item (cart_id, ad_id, seller_id) VALUES ($1, $2, $3)
    ON CONFLICT (cart_id, ad_id) DO NOTHING
`

const removeCartItemsQuery = `
    DELETE FROM cart_item ci
    USING cart c
    WHERE ci.cart_id = c.id AND c.user_id = $1 AND ci.ad_id = ANY($2)
`

const countCartQuery = `
    SELECT count(*)
    FROM cart_item ci
    JOIN cart c ON c.id = ci.cart_id
    WHERE c.user_id = $1
`
