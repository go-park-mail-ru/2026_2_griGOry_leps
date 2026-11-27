# Таблицы, которыми владеют сервисы после разделения БД. Подключается через source из split-db.sh.
AUTH_TABLES=(city user_account session password_reset email_verification)
ADS_TABLES=(
    category ad ad_image ad_status_event price_history moderation_decision
    cart cart_item address orders order_seller order_item
    review review_image seller_report
    favorites views promotion ad_reaction conversation message
)
