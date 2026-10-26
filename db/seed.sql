-- Тестовые данные, чтобы лента не была пустой. Запускать один раз на базу после миграций: make seed
-- Продавец: seller@goget.pw / Seller123, покупатели с отзывами: buyer1@goget.pw и buyer2@goget.pw с тем же паролем

BEGIN;

INSERT INTO category (category_id, name, slug) OVERRIDING SYSTEM VALUE VALUES
    (1, 'Авто', 'auto'),
    (2, 'Недвижимость', 'realty'),
    (3, 'Жильё для путешествий', 'rent'),
    (4, 'Для дома и дачи', 'home'),
    (5, 'Запчасти', 'parts'),
    (6, 'Услуги', 'services'),
    (7, 'Электроника', 'electronics'),
    (8, 'Работа и подработка', 'jobs'),
    (9, 'Бизнес 360', 'biznes'),
    (10, 'Одежда, обувь, аксессуары', 'clothes'),
    (11, 'Животные', 'pets'),
    (12, 'Детские товары', 'kids'),
    (13, 'Красота и здоровье', 'beauty'),
    (14, 'Спорт и отдых', 'sport'),
    (15, 'Книги и журналы', 'books'),
    (16, 'Хобби и фото', 'hobby')
ON CONFLICT DO NOTHING;

SELECT setval(pg_get_serial_sequence('category', 'category_id'), (SELECT max(category_id) FROM category));

WITH seller AS (
    INSERT INTO user_account (first_name, email, phone_number, password_hash, city_id, about)
    VALUES ('Анна', 'seller@goget.pw', '+79990000001', '$2a$10$gRYWtxyjUoUpvKEaBkyAduv598hcAGBmKSPThGDEDORdFfW.WKv7i',
            (SELECT city_id FROM city WHERE name = 'Москва'), 'Продаю вещи после переезда')
    ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
    RETURNING user_id
),
data (category_id, title, description, price, city, has_delivery, image, hours_ago) AS (VALUES
    (14, 'Велосипед горный, состояние отличное', 'Stels Navigator, 21 скорость', 1500000, 'Москва', false, '/img/listings/bike-mountain.jpg', 0),
    (14, 'Велосипед шоссейный, Б/У', 'Алюминиевая рама, размер M', 4500000, 'Москва', false, '/img/listings/bike-road.jpg', 3),
    (12, 'Рюкзак школьный ортопедический', 'Жёсткая спинка, светоотражатели', 320000, 'Химки', true, '/img/listings/backpack.jpg', 6),
    (4, 'Диван раскладной, доставка', 'Велюр, механизм еврокнижка', 4800000, 'Москва', true, '/img/listings/sofa.jpg', 9),
    (15, 'Комплект книг для 5 класса', 'Все учебники по программе', 150000, 'Балашиха', true, '/img/listings/school-books.jpg', 12),
    (7, 'Ноутбук игровой, гарантия', 'RTX 4060, 16 ГБ, гарантия до весны', 6200000, 'Москва', true, '/img/listings/laptop.jpg', 15),
    (14, 'Электросамокат Xiaomi, пробег 300 км', 'Родная зарядка, без ремонтов', 1850000, 'Мытищи', true, '/img/listings/scooter.jpg', 18),
    (14, 'Палатка 4-местная, двухслойная', 'Использовалась два похода', 690000, 'Москва', true, '/img/listings/tent.jpg', 21),
    (4, 'Кофемашина рожковая DeLonghi', 'Капучинатор, полный комплект', 1400000, 'Химки', true, '/img/listings/coffee-machine.jpg', 24),
    (16, 'Гитара акустическая Yamaha F310', 'Новые струны, чехол в подарок', 950000, 'Москва', false, '/img/listings/guitar.jpg', 27),
    (12, 'Коляска прогулочная, лёгкая', 'Складывается одной рукой', 780000, 'Балашиха', true, '/img/listings/stroller.jpg', 30),
    (7, 'PlayStation 5 с двумя джойстиками', 'Дисковая версия, на гарантии', 4200000, 'Москва', false, '/img/listings/ps5.jpg', 33)
),
inserted AS (
    INSERT INTO ad (user_id, category_id, title, description, price, city, has_delivery, status, created_at)
    SELECT seller.user_id, d.category_id, d.title, d.description, d.price, d.city, d.has_delivery,
           'active', now() - d.hours_ago * interval '1 hour'
    FROM data d CROSS JOIN seller
    RETURNING id, title
)
INSERT INTO ad_image (ad_id, url)
SELECT i.id, d.image
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
