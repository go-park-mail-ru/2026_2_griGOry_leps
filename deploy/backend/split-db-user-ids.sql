-- ID пользователей, на которые ссылаются данные ads. Каждый должен существовать в user_account сервиса auth.
SELECT DISTINCT id FROM (
    SELECT user_id AS id FROM ad
    UNION ALL SELECT user_id FROM ad_image
    UNION ALL SELECT actor_id FROM ad_status_event
    UNION ALL SELECT moderator_id FROM moderation_decision
    UNION ALL SELECT user_id FROM cart
    UNION ALL SELECT seller_id FROM cart_item
    UNION ALL SELECT user_id FROM address
    UNION ALL SELECT buyer_id FROM orders
    UNION ALL SELECT seller_id FROM order_seller
    UNION ALL SELECT seller_id FROM order_item
    UNION ALL SELECT user_id FROM review
    UNION ALL SELECT seller_id FROM review
    UNION ALL SELECT seller_id FROM seller_report
    UNION ALL SELECT reporter_id FROM seller_report
    UNION ALL SELECT user_id FROM favorites
    UNION ALL SELECT user_id FROM views
    UNION ALL SELECT user_id FROM ad_reaction
    UNION ALL SELECT buyer_id FROM conversation
    UNION ALL SELECT seller_id FROM conversation
    UNION ALL SELECT sender_id FROM message
) ids WHERE id IS NOT NULL ORDER BY id;
