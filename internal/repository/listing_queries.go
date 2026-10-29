package repository

const getListingQuery = `
    SELECT a.id, a.user_id, a.category_id, COALESCE(c.name, ''), COALESCE(cp.path, ''),
           a.title, COALESCE(a.description, ''), a.price, a.is_free, a.is_negotiable, a.condition,
           COALESCE(a.city, ''), a.address, a.has_delivery, a.status, a.close_reason,
           a.published_at, a.created_at, a.updated_at,
           u.first_name, COALESCE(u.avatar, ''), u.created_at,
           COALESCE(r.rating, 0), COALESCE(r.reviews_count, 0),
           EXISTS (
               SELECT 1
               FROM cart_item ci
               JOIN cart ct ON ct.id = ci.cart_id
               WHERE ct.user_id = $2 AND ci.ad_id = a.id
           )
    FROM ad a
    JOIN user_account u ON u.user_id = a.user_id
    LEFT JOIN category c ON c.category_id = a.category_id
    LEFT JOIN LATERAL (
        WITH RECURSIVE chain AS (
            SELECT category_id, parent_id, name, 0 AS depth
            FROM category
            WHERE category_id = a.category_id
            UNION ALL
            SELECT p.category_id, p.parent_id, p.name, chain.depth + 1
            FROM category p
            JOIN chain ON p.category_id = chain.parent_id
            WHERE chain.depth < 10
        )
        SELECT string_agg(name, ' › ' ORDER BY depth DESC) AS path FROM chain
    ) cp ON true
    LEFT JOIN LATERAL (
        SELECT ROUND(AVG(rv.rating), 2)::float8 AS rating, COUNT(*) AS reviews_count
        FROM review rv
        WHERE rv.seller_id = a.user_id
    ) r ON true
    WHERE a.id = $1
`

const listingPhotosQuery = `
    SELECT image_id, url
    FROM ad_image
    WHERE ad_id = $1
    ORDER BY position, image_id
`

const getListingStateForUpdateQuery = `
    SELECT user_id, status, category_id, title, COALESCE(description, ''), price,
           is_free, is_negotiable, condition, address, has_delivery
    FROM ad
    WHERE id = $1
    FOR UPDATE
`

const createListingQuery = `
    INSERT INTO ad (user_id, status, category_id, title, description, price,
                    is_free, is_negotiable, condition, address, has_delivery, city)
    VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, $9, $10, $11, (
        SELECT c.name
        FROM user_account u
        JOIN city c ON c.city_id = u.city_id
        WHERE u.user_id = $1
    ))
    RETURNING id
`

const updateListingQuery = `
    UPDATE ad
    SET status = $2, category_id = $3, title = $4, description = NULLIF($5, ''), price = $6,
        is_free = $7, is_negotiable = $8, condition = $9, address = $10, has_delivery = $11
    WHERE id = $1
`

const lockOwnPhotosQuery = `
    SELECT image_id
    FROM ad_image
    WHERE image_id = ANY($1) AND user_id = $2 AND (ad_id IS NULL OR ad_id = $3)
    FOR UPDATE
`

const detachMissingPhotosQuery = `
    DELETE FROM ad_image
    WHERE ad_id = $1 AND NOT (image_id = ANY($2))
    RETURNING url
`

const shiftPhotoPositionsQuery = `
    UPDATE ad_image
    SET position = position + 10000
    WHERE ad_id = $1
`

const attachPhotosQuery = `
    UPDATE ad_image i
    SET ad_id = $1, position = t.ord - 1
    FROM unnest($2::integer[]) WITH ORDINALITY AS t(id, ord)
    WHERE i.image_id = t.id
`

const deleteListingPhotosQuery = `
    SELECT url
    FROM ad_image
    WHERE ad_id = $1
`

const deleteListingQuery = `
    DELETE FROM ad
    WHERE id = $1
`

const closeListingQuery = `
    UPDATE ad
    SET status = 'closed', close_reason = $2
    WHERE id = $1
`

const isLeafCategoryQuery = `
    SELECT EXISTS (
        SELECT 1
        FROM category c
        WHERE c.category_id = $1
          AND NOT EXISTS (SELECT 1 FROM category ch WHERE ch.parent_id = c.category_id)
    )
`

const listCategoriesQuery = `
    SELECT category_id, parent_id, name, slug
    FROM category
    ORDER BY category_id
`

const createPhotoQuery = `
    INSERT INTO ad_image (user_id, url)
    VALUES ($1, $2)
    RETURNING image_id
`

const approveModerationQuery = `
    UPDATE ad
    SET status = 'active', published_at = COALESCE(published_at, now())
    WHERE status = 'moderation'
`

const deleteStalePhotosQuery = `
    DELETE FROM ad_image
    WHERE ad_id IS NULL AND created_at < $1
    RETURNING url
`
