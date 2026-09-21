# 2026_2_griGOry_leps

Backend-репозиторий проекта «Go&Get» команды «griGOry_leps» — маркетплейс объявлений (C2C).

## Ссылки

- [Доска задач (YouGile)](https://ru.yougile.com/team/82999ec3673c/GOSH4LEEPS)
- [Репозиторий фронтенда](https://github.com/frontend-park-mail-ru/2026_2_griGOry_leps)
- [Макеты в Figma]()
- [Deploy]()

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
- **Роутинг:** [chi](https://github.com/go-chi/chi)
- **База данных:** PostgreSQL
- **Авторизация:** cookie-сессии
- **Миграции:** [goose](https://github.com/pressly/goose)

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

- [Go](https://go.dev/dl/) 1.23+
- [Docker](https://www.docker.com/) и Docker Compose
- [goose](https://github.com/pressly/goose) для миграций: `go install github.com/pressly/goose/v3/cmd/goose@latest`

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

3. Поднимите базу данных:

   ```bash
   docker compose up -d
   ```

4. Установите зависимости и накатите миграции:

   ```bash
   go mod tidy
   make migrate-up
   ```

5. Запустите сервер:

   ```bash
   make run
   ```

Бэкенд будет доступен на `http://localhost:8080`.
