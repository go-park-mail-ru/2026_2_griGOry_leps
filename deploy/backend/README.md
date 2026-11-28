# Развёртывание GoGet на сервере

На сервере работают четыре независимые службы `goget-backend@auth`, `@ads`,
`@search` и `@onlineconf`. `goget-gateway` запускает их при загрузке системы и принимает
внешние запросы на существующем порту `8000`. Сайт и его настройки API
остаются `http://goget.pw:8001` и `http://goget.pw:8000/api`.

| Служба | Локальная проверка |
| --- | --- |
| auth | `127.0.0.1:8081/api/health`; gRPC на `127.0.0.1:9091` |
| ads | `127.0.0.1:8082/api/health`; gRPC на `127.0.0.1:9092` |
| search | `127.0.0.1:8083/api/health` |
| onlineconf | `127.0.0.1:8084/api/health` |
| gateway | `127.0.0.1:8000/api/health`, полный сценарий через `health.sh` |

## Подготовка

Команды выполняются на Linux от `ubuntu`, из `/home/ubuntu/backend`,
с Go из версии в `go.mod`. На сервере нужны `systemd`, `curl`, `jq`,
`flock` и действующий доступ к PostgreSQL.

Рабочим каталогом остаётся `/home/ubuntu/backend`. Сервисы берут секреты
и настройки приложения из существующего `.env`, включая `DATABASE_URL`,
`CSRF_SECRET`, SMTP, `FRONTEND_ORIGIN` и `PUBLIC_URL`. Сохраняйте
путь `UPLOAD_DIR`; по умолчанию это `/home/ubuntu/backend/uploads`.
Файл `/etc/goget/backend.runtime.env` задаёт порты и адреса на сервере.
Эти значения нужно менять вместе с адресами проверок в `health.sh`.
Пустой `GATEWAY_HOST` открывает API на всех интерфейсах. Для изолированного
запуска можно задать `GATEWAY_HOST=127.0.0.1`, отдельную БД и свободные порты.

Перед активацией проверьте версию схемы БД и примените необходимые для
выбранного коммита миграции отдельно. Перед первой активацией OnlineConf
обязательно примените миграцию 15 (`15_onlineconf_admin.sql`). Активация переключает бинарники
и службы. При запуске onlineconf проверяет доступность своей таблицы
с таймаутом 5 секунд; без миграции сервер не стартует. Откат кода возможен, пока предыдущая версия совместима со схемой.

## Установка служб

Один раз, после получения проверенного коммита:

```bash
sudo install -d -m 0755 /etc/goget /opt/goget/backend/{bin,releases}
sudo install -m 0644 deploy/backend/backend.runtime.env /etc/goget/
sudo install -m 0644 deploy/backend/*.service /etc/systemd/system/
sudo install -D -m 0644 deploy/backend/goget-backend@onlineconf.service.d/10-release.conf \
    /etc/systemd/system/goget-backend@onlineconf.service.d/10-release.conf
sudo install -m 0755 deploy/backend/{activate,health}.sh /opt/goget/backend/bin/
sudo systemctl daemon-reload
```

При следующих изменениях этих файлов повторите установку. Релиз с OnlineConf
меняет службы и оба скрипта, поэтому перед его первой активацией сначала
повторите установку служб: со старыми `activate.sh` и `health.sh` onlineconf не
запустится, проверки пройдут, а `/api/onlineconf/*` будет отвечать 502. Службы сами
перезапускаются через три секунды при выходе процесса. Отдельный перезапуск
одной службы не перезапускает gateway и остальные службы.

## Сборка и переключение версии

Из чистого checkout проверенного коммита:

```bash
release=$(git rev-parse --short=12 HEAD)
build_dir=$(mktemp -d)
go build -trimpath -o "$build_dir/" ./cmd/auth ./cmd/ads ./cmd/search ./cmd/onlineconf ./cmd/gateway
sudo mkdir "/opt/goget/backend/releases/$release"
sudo install -m 0755 "$build_dir"/{auth,ads,search,onlineconf,gateway} "/opt/goget/backend/releases/$release/"
rm -f "$build_dir"/{auth,ads,search,onlineconf,gateway}
rmdir "$build_dir"
sudo /opt/goget/backend/bin/activate.sh "$release"
```

Используйте существующий каталог релиза для повторного включения той же
версии. Если команда сборки завершилась с ошибкой, остановитесь перед
установкой файлов. Для сборки Go на этом сервере может понадобиться
`export PATH="/usr/local/go/bin:$PATH"`.

При первом переключении скрипт останавливает монолитный
`gogetbackend.service`, чтобы освободить порт `8000`. После успешных
проверок он отключает автозапуск монолита и включает автозапуск gateway.
При ошибке запуска, неуспешной проверке или прерывании активации сигналом
`SIGTERM`, `SIGINT` или `SIGHUP` скрипт возвращает прошлый релиз либо
монолит. Для переключения на предыдущий релиз запустите
`activate.sh` с его идентификатором. Для старого релиза без бинарника
`onlineconf` явно укажите `sudo /opt/goget/backend/bin/activate.sh --legacy RELEASE`;
для его ручной проверки используйте `health.sh --legacy`.
Если историческая версия возвращает 501 от заглушки поиска, используйте
`sudo env ALLOW_SEARCH_STUB=1 /opt/goget/backend/bin/activate.sh --legacy RELEASE`
и `ALLOW_SEARCH_STUB=1 /opt/goget/backend/bin/health.sh --legacy`. Обычная активация
требует все пять бинарников и не принимает неполную новую поставку.
При неуспешной активации состав служб и проверок восстанавливается автоматически.
Drop-in `goget-backend@onlineconf.service.d/10-release.conf` пропускает запуск
onlineconf по `ConditionFileIsExecutable`, если в текущем релизе нет его
бинарника, поэтому зависимость gateway не запускает OnlineConf после отката и
перезагрузки. Для auth, ads и search условия нет: без бинарника служба падает.

Если после первого успешного переключения нужно вручную вернуться к
сохранённому монолиту, выполните на сервере:

```bash
sudo systemctl disable --now goget-gateway.service
sudo systemctl stop goget-backend@{auth,ads,search,onlineconf}.service
sudo systemctl enable --now gogetbackend.service
curl -fsS http://127.0.0.1:8000/api/health
```

## Проверки и журналы

```bash
/opt/goget/backend/bin/health.sh
systemctl status goget-backend@{auth,ads,search,onlineconf}.service goget-gateway.service
sudo systemctl restart goget-backend@search.service
journalctl -u goget-backend@search.service -n 50 --no-pager
```

`health.sh` проверяет каждый сервис напрямую, а через gateway проверяет
запросы к PostgreSQL, вызов auth по gRPC из ads и onlineconf и вызов ads по gRPC из поиска.
При автоматическом откате к предыдущему релизу допустим `501` от старой
заглушки поиска.

## Первый администратор OnlineConf

После миграции 15 выдайте право пользователю GoGet. `N` — поле `id` ответа
`GET /api/me`, а не ID сессии. Команда идемпотентна; существование аккаунта
не проверяется. CLI сам загружает `.env` из рабочего каталога.
Команда не требует `CSRF_SECRET`, поскольку выполняется локально:

```bash
cd /home/ubuntu/backend
/opt/goget/backend/current/onlineconf grant-admin --user-id N
```

Успех: `granted user N`; повтор: `user N is already an admin`, код выхода 0.
Ошибка аргументов или БД — код 1; неизвестная подкоманда — код 2.
С действующей сессией, но без права `/api/onlineconf/admin/me` возвращает 403.
Для отзыва удалите строку `onlineconf_admin` с нужным `user_id`; право
проверяется при каждом запросе и не кешируется.

Регрессионные проверки скриптов без systemd и PostgreSQL:
`python3 deploy/backend/deploy_test.py` (нужен Python 3). Тесты используют
временные каталоги и подменяют системные команды.
