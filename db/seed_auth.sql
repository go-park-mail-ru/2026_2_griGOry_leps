-- Тестовые аккаунты сервиса auth. Запускать один раз после миграций: make seed
-- Продавец: seller@goget.pw / Seller123, покупатели: buyer1@goget.pw и buyer2@goget.pw с тем же паролем.
-- ID заданы явно: db/seed_ads.sql ссылается на них (продавец 1, покупатели 2 и 3).

BEGIN;

INSERT INTO user_account (user_id, first_name, email, phone_number, password_hash, city_id, about)
OVERRIDING SYSTEM VALUE
VALUES
    (1, 'Анна', 'seller@goget.pw', '+79990000001', '$2a$10$gRYWtxyjUoUpvKEaBkyAduv598hcAGBmKSPThGDEDORdFfW.WKv7i',
     (SELECT city_id FROM city WHERE name = 'Москва'), 'Продаю вещи после переезда')
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO user_account (user_id, first_name, email, phone_number, password_hash)
OVERRIDING SYSTEM VALUE
VALUES
    (2, 'Ольга', 'buyer1@goget.pw', '+79990000002', '$2a$10$gRYWtxyjUoUpvKEaBkyAduv598hcAGBmKSPThGDEDORdFfW.WKv7i'),
    (3, 'Пётр', 'buyer2@goget.pw', '+79990000003', '$2a$10$gRYWtxyjUoUpvKEaBkyAduv598hcAGBmKSPThGDEDORdFfW.WKv7i')
ON CONFLICT (user_id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('user_account', 'user_id'), (SELECT max(user_id) FROM user_account));

COMMIT;
