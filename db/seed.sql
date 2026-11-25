-- Тестовые данные, чтобы лента не была пустой. Запускать один раз на базу после миграций: make seed
-- Продавец: seller@goget.pw / Seller123, покупатели с отзывами: buyer1@goget.pw и buyer2@goget.pw с тем же паролем

BEGIN;

WITH seller AS (
    INSERT INTO user_account (first_name, email, phone_number, password_hash, city_id, about)
    VALUES ('Анна', 'seller@goget.pw', '+79990000001', '$2a$10$gRYWtxyjUoUpvKEaBkyAduv598hcAGBmKSPThGDEDORdFfW.WKv7i',
            (SELECT city_id FROM city WHERE name = 'Москва'), 'Продаю вещи после переезда')
    ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
    RETURNING user_id
),
data (category_slug, title, description, price, city, has_delivery, image, hours_ago) AS (VALUES
    ('sport-bikes', 'Велосипед горный, состояние отличное', 'Stels Navigator, 21 скорость', 1500000, 'Москва', false, '/img/listings/bike-mountain.jpg', 0),
    ('sport-bikes', 'Велосипед шоссейный, Б/У', 'Алюминиевая рама, размер M', 4500000, 'Москва', false, '/img/listings/bike-road.jpg', 3),
    ('kids', 'Рюкзак школьный ортопедический', 'Жёсткая спинка, светоотражатели', 320000, 'Химки', true, '/img/listings/backpack.jpg', 6),
    ('home-furniture', 'Диван раскладной, доставка', 'Велюр, механизм еврокнижка', 4800000, 'Москва', true, '/img/listings/sofa.jpg', 9),
    ('books-textbooks', 'Комплект книг для 5 класса', 'Все учебники по программе', 150000, 'Балашиха', true, '/img/listings/school-books.jpg', 12),
    ('electronics-laptops', 'Ноутбук игровой, гарантия', 'RTX 4060, 16 ГБ, гарантия до весны', 6200000, 'Москва', true, '/img/listings/laptop.jpg', 15),
    ('sport-scooters', 'Электросамокат Xiaomi, пробег 300 км', 'Родная зарядка, без ремонтов', 1850000, 'Мытищи', true, '/img/listings/scooter.jpg', 18),
    ('sport-camping', 'Палатка 4-местная, двухслойная', 'Использовалась два похода', 690000, 'Москва', true, '/img/listings/tent.jpg', 21),
    ('home-appliances', 'Кофемашина рожковая DeLonghi', 'Капучинатор, полный комплект', 1400000, 'Химки', true, '/img/listings/coffee-machine.jpg', 24),
    ('hobby-instruments', 'Гитара акустическая Yamaha F310', 'Новые струны, чехол в подарок', 950000, 'Москва', false, '/img/listings/guitar.jpg', 27),
    ('kids-strollers', 'Коляска прогулочная, лёгкая', 'Складывается одной рукой', 780000, 'Балашиха', true, '/img/listings/stroller.jpg', 30),
    ('electronics-gaming', 'PlayStation 5 с двумя джойстиками', 'Дисковая версия, на гарантии', 4200000, 'Москва', false, '/img/listings/ps5.jpg', 33)
),
inserted AS (
    INSERT INTO ad (user_id, category_id, title, description, price, city, address, condition, has_delivery,
                    status, created_at, published_at)
    SELECT seller.user_id, c.category_id, d.title, d.description, d.price, d.city, d.city, 'used_good', d.has_delivery,
           'active', now() - d.hours_ago * interval '1 hour', now() - d.hours_ago * interval '1 hour'
    FROM data d JOIN category c ON c.slug = d.category_slug CROSS JOIN seller
    RETURNING id, user_id, title
)
INSERT INTO ad_image (ad_id, user_id, url)
SELECT i.id, i.user_id, d.image
FROM inserted i
JOIN data d ON d.title = i.title;

INSERT INTO user_account (first_name, email, phone_number, password_hash)
VALUES ('Ольга', 'buyer1@goget.pw', '+79990000002', '$2a$10$gRYWtxyjUoUpvKEaBkyAduv598hcAGBmKSPThGDEDORdFfW.WKv7i'),
       ('Пётр', 'buyer2@goget.pw', '+79990000003', '$2a$10$gRYWtxyjUoUpvKEaBkyAduv598hcAGBmKSPThGDEDORdFfW.WKv7i')
ON CONFLICT (email) DO NOTHING;

INSERT INTO review (user_id, ad_id, text, rating, created_at)
SELECT b.user_id, a.id, r.text, r.rating, now() - r.days_ago * interval '1 day'
FROM (VALUES
    ('buyer1@goget.pw', 'Велосипед горный, состояние отличное', 'Всё как в описании, продавец на связи', 5, 2),
    ('buyer2@goget.pw', 'Велосипед горный, состояние отличное', 'Пришлось подождать, но в итоге всё хорошо', 4, 5),
    ('buyer1@goget.pw', 'Палатка 4-местная, двухслойная', 'Палатка с небольшой дыркой, о ней не предупредили', 3, 9)
) AS r (email, title, text, rating, days_ago)
JOIN user_account b ON b.email = r.email
JOIN ad a ON a.title = r.title
JOIN user_account s ON s.user_id = a.user_id AND s.email = 'seller@goget.pw'
ON CONFLICT (user_id, ad_id) DO NOTHING;

COMMIT;
