# Схема базы данных Go&Get

ДЗ1 по СУБД: сущности, ER-диаграмма и DDL. Полный SQL со всеми таблицами, индексами, функциями и триггерами лежит в [schema.sql](schema.sql).

На РК1 бэкенд хранит данные в памяти процесса, эта схема будет использоваться с подключением PostgreSQL.

## Сущности

- **USER_ACCOUNT** — продавец или покупатель на платформе.
- **CATEGORY** — категория объявления. Поддерживает иерархию через ссылку на родительскую категорию (`parent_id`).
- **AD** — карточка товара и её данные: категория (`category_id`), город (`city`) и признак возможности доставки (`has_delivery`).
- **AD_IMAGE** — изображение объявления с порядком отображения (`position`).
- **CART** — корзина покупателя.
- **CART_ITEM** — объявление, лежащее в корзине покупателя.
- **FAVORITES** — избранные объявления пользователя.
- **AD_STATUS_EVENT** — история изменения статуса объявления.
- **PRICE_HISTORY** — история изменения цены объявления.
- **CONVERSATION** — комната, где идёт диалог покупателя и продавца.
- **MESSAGE** — сообщение пользователя и его статус.
- **VIEWS** — просмотр объявления.
- **REVIEW** — отзыв пользователя под объявлением.
- **PROMOTION** — вид и срок платного продвижения объявления.
- **LIKE** — одобрение объявления покупателем.

## ER-диаграмма

```mermaid
erDiagram
    USER_ACCOUNT {
        int user_id PK
        varchar nickname
        varchar first_name
        varchar last_name
        varchar email
        varchar phone_number
        date birthday
        varchar password_hash
        timestamptz created_at
        timestamptz updated_at
        varchar type
    }

    CATEGORY {
        int category_id PK
        int parent_id FK
        varchar name
        varchar slug
        text image_url
        timestamptz created_at
        timestamptz updated_at
    }

    AD {
        int id PK
        varchar title
        text description
        numeric price
        int user_id FK
        timestamptz created_at
        timestamptz updated_at
        varchar status
        int category_id FK
        varchar type
        varchar city
        boolean has_delivery
    }

    AD_IMAGE {
        int image_id PK
        int ad_id FK
        text url
        int position
        varchar alt_text
        timestamptz created_at
    }

    CART {
        int id PK
        int user_id FK
        timestamptz created_at
    }

    CART_ITEM {
        int id PK
        int cart_id FK
        int ad_id FK
    }

    FAVORITES {
        int id PK
        int user_id FK
        int ad_id FK
    }

    AD_STATUS_EVENT {
        int stat_id PK
        int ad_id FK
        int actor_id FK
        varchar status
        timestamptz archived_at
    }

    PRICE_HISTORY {
        int prh_id PK
        int ad_id FK
        numeric price
        timestamptz changed_at
    }

    CONVERSATION {
        int room_id PK
        int ad_id FK
        int buyer_id FK
        int seller_id FK
        timestamptz created_at
    }

    MESSAGE {
        int message_id PK
        int sender_id FK
        int room_id FK
        text text
        timestamptz sent_at
        varchar status
    }

    VIEWS {
        int views_id PK
        int user_id FK
        int ad_id FK
        timestamptz view_at
    }

    REVIEW {
        int rew_id PK
        int user_id FK
        int ad_id FK
        text text
        numeric rating
        timestamptz created_at
        timestamptz updated_at
    }

    PROMOTION {
        int prom_id PK
        int ad_id FK
        timestamptz when_paid
        date duration
        int promotion_level
    }

    LIKE {
        int like_id PK
        int user_id FK
        int ad_id FK
        boolean is_like
    }

    USER_ACCOUNT ||--o{ AD : creates
    USER_ACCOUNT ||--|| CART : owns
    CART ||--o{ CART_ITEM : contains
    AD ||--o{ CART_ITEM : "added to"
    USER_ACCOUNT ||--o{ FAVORITES : adds
    AD ||--o{ FAVORITES : "marked in"
    AD ||--o{ AD_STATUS_EVENT : "status history"
    USER_ACCOUNT |o--o{ AD_STATUS_EVENT : changes
    AD ||--o{ PRICE_HISTORY : "price history"
    AD |o--o{ CONVERSATION : "discussed in"
    USER_ACCOUNT ||--o{ CONVERSATION : "buyer in"
    USER_ACCOUNT |o--o{ CONVERSATION : "seller in"
    CONVERSATION ||--o{ MESSAGE : contains
    USER_ACCOUNT ||--o{ MESSAGE : sends
    AD ||--o{ VIEWS : "viewed in"
    USER_ACCOUNT |o--o{ VIEWS : views
    USER_ACCOUNT ||--o{ REVIEW : writes
    AD ||--o{ REVIEW : has
    AD ||--o{ PROMOTION : "promoted by"
    USER_ACCOUNT ||--o{ LIKE : likes
    AD ||--o{ LIKE : "liked by"
    CATEGORY |o--o{ CATEGORY : parent
    CATEGORY |o--o{ AD : classifies
    AD ||--o{ AD_IMAGE : has
```
