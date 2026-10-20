# 2026_2_griGOry_leps

Backend-репозиторий проекта «Go&Get» команды «griGOry_leps» — маркетплейс объявлений (C2C).

## Ссылки

- [Доска задач (YouGile)](https://ru.yougile.com/team/82999ec3673c/GOSH4LEEPS)
- [Репозиторий фронтенда](https://github.com/frontend-park-mail-ru/2026_2_griGOry_leps)
- [Макеты в Figma](https://www.figma.com/design/azXAN6gTvGAovlElu3VP3b/GoGET-%E2%80%94-Team-Workspace)
- [Сайт на проде](http://goget.pw:8001)
- [API на проде](http://goget.pw:8000/api/health)

> [!NOTE]
> Сайт открывается только по домену `goget.pw`. По IP фронт загрузится, но запросы
> к API не пройдут: бэк разрешает CORS и cookie только для `http://goget.pw:8001`.

## Участники команды

1. [Тимур Ведмецкий](https://github.com/v0rdYT)
2. [Александр Юнцевич](https://github.com/impduckzzz)
3. [Алиса Никитина](https://github.com/AlisaNikitinaa)
4. [Максим Полтинин](https://github.com/reynyng)

## Менторы

- [Арсений](https://t.me/arseniksk) — _Backend_
- [Лёша](https://t.me/Valekirrr) — _Frontend_
- [Миша](https://t.me/mx1262) — _BD_
- [Даниил](https://t.me/Daniil_Sentery) — _UX/UI_

## Технологический стек

- **Язык:** Go
- **Роутинг:** [gorilla/mux](https://github.com/gorilla/mux)
- **База данных:** PostgreSQL ([pgx](https://github.com/jackc/pgx)), миграции через [goose](https://github.com/pressly/goose) в папке `migrations/`
- **Авторизация:** cookie-сессии
- **Документация API:** OpenAPI ([`openapi.yaml`](openapi.yaml))

## Как работать с задачами

Все задачи, фронтовые и бэковые, ведутся на одной [доске в YouGile](https://ru.yougile.com/team/82999ec3673c/GOSH4LEEPS).

> [!IMPORTANT]
> Название ветки и Pull Request всегда содержат номер задачи (`GOS-###`) —
> это единственное, что связывает код с задачей на доске, так как задача
> и репозиторий живут в разных системах.

1. **Взять задачу.** На доске в YouGile выбрать задачу, назначить на себя, отметить, что взяли в работу

2. **Создать ветку** от `main` с именем `GOS-###`, где `###` — номер задачи:

   ```bash
   git checkout main && git pull
   git checkout -b GOS-12
   ```

3. **Закоммитить** по шаблону `<тип>: <описание>`, типы — в таблице ниже.
   Область в скобках после типа указывать необязательно:

   ```
   feat: добавить регистрацию
   fix: не сбрасывать сессию при обновлении страницы
   refactor(auth): вынести валидацию в отдельный модуль
   ```

4. **Открыть Pull Request** в `main`, когда код готов к ревью.
   Заголовок — по шаблону `GOS-###: description`, например `GOS-12: Регистрация и авторизация`.
   В описании PR — ссылка на задачу в YouGile

5. **Получить апрув** от тимлида/ментора

6. **Влить в `main`** через Merge, задачу в YouGile перевести в «Готово» вручную

## Типы коммитов

> [!NOTE]
> Коммиты ветки попадают в `main` как есть, поэтому от их качества зависит
> читаемость истории проекта. Сверху добавляется merge-коммит с названием Pull Request

| Тип | Когда используется |
|---|---|
| `feat` | новая функциональность |
| `fix` | исправление бага |
| `refactor` | код переписан, поведение не изменилось |
| `style` | форматирование и отступы, логика не тронута |
| `test` | тесты |
| `docs` | документация |
| `chore` | конфиги, зависимости, сборка, CI |

## Установка и запуск проекта

### Системные требования

- [Go](https://go.dev/dl/) 1.26+
- [PostgreSQL](https://www.postgresql.org/download/) 16+
- goose: `go install github.com/pressly/goose/v3/cmd/goose@latest`

### Пошаговая инструкция

1. Клонируйте репозиторий:

   ```bash
   git clone https://github.com/go-park-mail-ru/2026_2_griGOry_leps.git
   cd 2026_2_griGOry_leps
   ```

2. Создайте локальный файл окружения:

   ```bash
   cp .env.example .env
   ```

   | Переменная | Что задаёт | По умолчанию |
   |---|---|---|
   | `PORT` | порт сервера | `8080` |
   | `FRONTEND_ORIGIN` | адрес фронта, с которого разрешены запросы (CORS). Должен совпадать с адресом в браузере вплоть до порта | `http://localhost:5173` |
   | `COOKIE_SECURE` | флаг `Secure` у cookie сессии, `true` только при HTTPS | `false` |
   | `LOG_LEVEL` | минимальный уровень логов: `debug`, `info`, `warn`, `error`. На `debug` в лог пишутся и запросы к базе | `info` |
   | `CSRF_SECRET` | ключ подписи CSRF-токенов. Если не задан, генерируется при старте и токены сбрасываются при перезапуске | случайный |
   | `DATABASE_URL` | адрес базы | `postgres://postgres:12345@127.0.0.1:5432/goget?sslmode=disable` |
   | `UPLOAD_DIR` | папка для загруженных файлов (аватарок), создаётся сама | `uploads` |
   | `TEST_DATABASE_URL` | отдельная база для интеграционных тестов, только для `make test-integration` | — |
   | `GOOSE_DRIVER` | драйвер базы для goose | `postgres` |
   | `GOOSE_DBSTRING` | адрес базы для миграций, по умолчанию тот же, что `DATABASE_URL` | `${DATABASE_URL}` |

3. Установите зависимости:

   ```bash
   go mod download
   ```

4. Создайте базу, накатите миграции и тестовые объявления (логин, пароль и имя базы — как в `DATABASE_URL`):

   ```bash
   createdb -U postgres goget
   make migrate-up
   make seed
   ```

   `make seed` запускается один раз: он добавляет категории, продавца `seller@goget.pw` с паролем `Seller123` и 12 объявлений.

5. Запустите сервер:

   ```bash
   make run
   ```

Бэкенд будет доступен на `http://localhost:8080`, проверить можно запросом `curl http://localhost:8080/api/health`.

### Команды

| Команда | Что делает |
|---|---|
| `make run` | запуск сервера |
| `make build` | сборка бинарника в `bin/server` |
| `make test` | юнит-тесты, база не нужна |
| `make test-integration` | тесты репозиториев на настоящей базе из `TEST_DATABASE_URL` |
| `make tidy` | обновление `go.mod` и `go.sum` |
| `make migrate-up` | накатить все миграции |
| `make migrate-down` | откатить последнюю миграцию |
| `make migrate-status` | какие миграции уже применены |
| `make seed` | тестовые категории, продавец и объявления |

### Тесты

Юнит-тесты (`make test`) работают без базы. Тесты репозиториев помечены тегом `integration` и ходят в настоящий PostgreSQL. Каждый тест создаёт себе отдельную схему, накатывает в неё все миграции и удаляет схему после себя, поэтому данные в базе не пересекаются. Для них нужна отдельная пустая база:

```bash
createdb -U postgres goget_test
make test-integration
```

В CI такая база поднимается в контейнере, см. `.github/workflows/ci.yml`.

## API

Все ручки начинаются с `/api`, полное описание — в [`openapi.yaml`](openapi.yaml).

POST/PUT/PATCH/DELETE требуют заголовок `X-CSRF-Token` с токеном из `GET /api/csrf`, иначе 403. Токен привязан к сессии, поэтому после входа, регистрации и выхода его нужно запросить заново.

| Метод | Путь | Что делает |
|---|---|---|
| `GET` | `/api/health` | проверка, что сервер жив |
| `GET` | `/api/csrf` | CSRF-токен для изменяющих запросов |
| `POST` | `/api/register` | регистрация, сразу создаёт сессию |
| `POST` | `/api/login` | вход |
| `POST` | `/api/logout` | выход, удаляет сессию |
| `GET` | `/api/me` | профиль текущего пользователя |
| `PATCH` | `/api/me` | изменить имя, телефон, email, город или «о себе» |
| `PUT` | `/api/me/password` | сменить пароль, остальные сессии завершаются |
| `POST` | `/api/me/avatar` | загрузить аватарку (JPG или PNG до 5 МБ, `multipart/form-data`, поле `file`) |
| `GET` | `/api/listings` | лента объявлений |
| `GET` | `/uploads/avatars/{file}` | загруженная аватарка |

Ручки `/api/me*` без действующей cookie сессии отвечают 401. В `avatar_url` приходит путь на сервере API (`/uploads/avatars/…`), полный адрес картинки — origin API плюс этот путь.

## Структура проекта

```
cmd/server/          точка входа
internal/config/     чтение переменных окружения
internal/domain/     модели и ошибки
internal/delivery/   HTTP-обработчики, роутер, мидлвары и интерфейсы usecase
internal/logger/     JSON-логгер (slog) с request_id из контекста
internal/csrf/       выпуск и проверка CSRF-токенов (HMAC)
internal/usecase/    бизнес-логика и интерфейсы репозиториев
internal/repository/ репозитории на PostgreSQL, транзакции, лог запросов к базе, хранилище файлов
migrations/          миграции goose
db/                  описание схемы и тестовые данные (seed.sql)
```

## Деплой

Прод крутится на VPS: бэк на порту 8000, фронт на 8001, PostgreSQL слушает только localhost. Код лежит в `~/backend` и `~/frontend`, оба запущены как systemd-сервисы `gogetbackend` и `gogetfrontend`. Бэк читает переменные из `~/backend/.env`, аватарки складывает в `~/backend/uploads`.

Обновить бэк:

```bash
cd ~/backend
git pull
set -a; . ./.env; set +a
goose -dir migrations postgres "$DATABASE_URL" up
go build -o bin/server ./cmd/server
sudo systemctl restart gogetbackend
curl -s localhost:8000/api/health
```

Логи: `journalctl -u gogetbackend -n 50 --no-pager`.
