package repository

const reservedByOthers = `
    EXISTS (
        SELECT 1
        FROM order_item oi
        JOIN orders o ON o.id = oi.order_id
        WHERE oi.ad_id = a.id
          AND o.status = 'awaiting_payment'
          AND o.pay_until > now()
          AND (o.buyer_id <> $1 OR o.payment_status = 'processing')
    )
`

const cartEntryColumns = `
    a.id, a.title, a.price, (
        SELECT url FROM ad_image
        WHERE ad_id = a.id
        ORDER BY position, image_id
        LIMIT 1
    ), a.address, a.has_delivery, a.status, ` + reservedByOthers + `,
    u.user_id, u.first_name, COALESCE(u.avatar, ''), COALESCE(stats.rating, 0)
`

const cartSellerJoin = `
    JOIN user_account u ON u.user_id = a.user_id
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
    JOIN ad a ON a.id = ci.ad_id
` + cartSellerJoin + `
    WHERE c.user_id = $1
    ORDER BY ci.id DESC
`

const cartEntriesByIDsQuery = `
    SELECT ` + cartEntryColumns + `
    FROM cart c
    JOIN cart_item ci ON ci.cart_id = c.id
    JOIN ad a ON a.id = ci.ad_id
` + cartSellerJoin + `
    WHERE c.user_id = $1 AND a.id = ANY($2)
    ORDER BY ci.id DESC
`

const lockAdsQuery = `
    SELECT id FROM ad WHERE id = ANY($1) ORDER BY id FOR UPDATE
`

const getAdForCartQuery = `
    SELECT a.user_id, a.status, ` + reservedByOthers + `
    FROM ad a
    WHERE a.id = $2
`

const ensureCartQuery = `
    INSERT INTO cart (user_id) VALUES ($1)
    ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
    RETURNING id
`

const addCartItemQuery = `
    INSERT INTO cart_item (cart_id, ad_id) VALUES ($1, $2)
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
