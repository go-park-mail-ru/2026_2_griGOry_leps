# Развёртывание GoGet на сервере

На сервере работают три независимые службы `goget-backend@auth`, `@ads`
и `@search`. `goget-gateway` запускает их при загрузке системы и принимает
внешние запросы на существующем порту `8000`. Сайт и его настройки API
остаются `http://goget.pw:8001` и `http://goget.pw:8000/api`.

| Служба | Локальная проверка |
| --- | --- |
| auth | `127.0.0.1:8081/api/health`; gRPC на `127.0.0.1:9091` |
| ads | `127.0.0.1:8082/api/health`; gRPC на `127.0.0.1:9092` |
| search | `127.0.0.1:8083/api/health` |
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
выбранного коммита миграции отдельно. Активация переключает бинарники
и службы. Откат кода возможен, пока предыдущая версия совместима со схемой.

## Установка служб

Один раз, после получения проверенного коммита:

```bash
sudo install -d -m 0755 /etc/goget /opt/goget/backend/{bin,releases}
sudo install -m 0644 deploy/backend/backend.runtime.env /etc/goget/
sudo install -m 0644 deploy/backend/*.service /etc/systemd/system/
sudo install -m 0755 deploy/backend/{activate,health}.sh /opt/goget/backend/bin/
sudo systemctl daemon-reload
```

При следующих изменениях этих файлов повторите установку. Службы сами
перезапускаются через три секунды при выходе процесса. Отдельный перезапуск
одной службы не перезапускает gateway и остальные службы.

## Сборка и переключение версии

Из чистого checkout проверенного коммита:

```bash
release=$(git rev-parse --short=12 HEAD)
build_dir=$(mktemp -d)
go build -trimpath -o "$build_dir/" ./cmd/auth ./cmd/ads ./cmd/search ./cmd/gateway
sudo mkdir "/opt/goget/backend/releases/$release"
sudo install -m 0755 "$build_dir"/{auth,ads,search,gateway} "/opt/goget/backend/releases/$release/"
rm -f "$build_dir"/{auth,ads,search,gateway}
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
`activate.sh` с его идентификатором.

Если после первого успешного переключения нужно вручную вернуться к
сохранённому монолиту, выполните на сервере:

```bash
sudo systemctl disable --now goget-gateway.service
sudo systemctl stop goget-backend@{auth,ads,search}.service
sudo systemctl enable --now gogetbackend.service
curl -fsS http://127.0.0.1:8000/api/health
```

## Проверки и журналы

```bash
/opt/goget/backend/bin/health.sh
systemctl status goget-backend@{auth,ads,search}.service goget-gateway.service
sudo systemctl restart goget-backend@search.service
journalctl -u goget-backend@search.service -n 50 --no-pager
```

`health.sh` проверяет каждый сервис напрямую, а через gateway проверяет
запросы к PostgreSQL, вызов auth по gRPC из ads и вызов ads по gRPC из поиска.
При автоматическом откате к предыдущему релизу допустим `501` от старой
заглушки поиска.
