package repository

const lockBuyerQuery = `
    SELECT pg_advisory_xact_lock(hashtext('orders'), $1)
`

const orderIDByKeyQuery = `
    SELECT id FROM orders WHERE buyer_id = $1 AND idempotency_key = $2
`

const cancelOwnOverlappingQuery = `
    UPDATE orders
    SET status = 'cancelled', updated_at = now()
    WHERE buyer_id = $1
      AND status = 'awaiting_payment'
      AND payment_status <> 'processing'
      AND id IN (SELECT order_id FROM order_item WHERE ad_id = ANY($2))
`

const createOrderQuery = `
    INSERT INTO orders (buyer_id, idempotency_key, status, payment_method, payment_status, payment_token,
                        pay_until, recipient_name, recipient_phone, address_id, address_text, delivery_date,
                        time_slot, comment, items_total, delivery_total)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::date, $13, $14, $15, $16)
    RETURNING id
`

const createOrderSellersQuery = `
    INSERT INTO order_seller (order_id, seller_id, method, price)
    SELECT $1, s.seller_id, s.method, s.price
    FROM unnest($2::integer[], $3::text[], $4::bigint[]) AS s(seller_id, method, price)
`

const createOrderItemsQuery = `
    INSERT INTO order_item (order_id, ad_id, seller_id, title, price, image_url)
    SELECT $1, i.ad_id, i.seller_id, i.title, i.price, i.image_url
    FROM unnest($2::integer[], $3::integer[], $4::text[], $5::bigint[], $6::text[])
         WITH ORDINALITY AS i(ad_id, seller_id, title, price, image_url, ord)
    ORDER BY i.ord
`

const getOrderQuery = `
    SELECT id, buyer_id, status, payment_method, payment_status, payment_token, payment_confirm_at, pay_until,
           recipient_name, recipient_phone, address_id, address_text, to_char(delivery_date, 'YYYY-MM-DD'),
           time_slot, comment, items_total, delivery_total, created_at
    FROM orders
    WHERE id = $1
`

const orderSellersQuery = `
    SELECT seller_id, method, price FROM order_seller WHERE order_id = $1 ORDER BY seller_id
`

const orderItemsQuery = `
    SELECT ad_id, seller_id, title, price, image_url FROM order_item WHERE order_id = $1 ORDER BY id
`

const orderIDByTokenQuery = `
    SELECT id FROM orders WHERE payment_token = $1
`

const updateOrderPaymentQuery = `
    UPDATE orders
    SET status = $2, payment_method = $3, payment_status = $4, payment_token = $5, payment_confirm_at = $6,
        updated_at = now()
    WHERE id = $1
`

const markOrderSoldQuery = `
    UPDATE ad SET status = 'sold', updated_at = now()
    WHERE id IN (SELECT ad_id FROM order_item WHERE order_id = $1 AND ad_id IS NOT NULL)
`

const dueOrdersQuery = `
    SELECT id FROM orders
    WHERE status = 'awaiting_payment'
      AND ((payment_status = 'processing' AND payment_confirm_at <= $1) OR pay_until <= $1)
    ORDER BY id
`

const activeOrderForListingQuery = `
    SELECT o.id
    FROM order_item oi
    JOIN orders o ON o.id = oi.order_id
    WHERE oi.ad_id = $1
      AND (o.status = 'placed' OR (o.status = 'awaiting_payment'
           AND (o.payment_status = 'processing' OR o.pay_until > now())))
    ORDER BY o.id DESC
    LIMIT 1
`
