# Мониторинг GoGet

Prometheus каждые 15 секунд опрашивает `/metrics` четырёх сервисов и gateway, Node
Exporter и собственный endpoint. Сервисы отдают стандартные метрики Go
и процесса, Node Exporter — метрики хоста. Имена jobs в Prometheus:
`auth`, `ads`, `search`, `onlineconf`, `gateway`, `node`, `prometheus`.

## Установка на сервере

Команды рассчитаны на Ubuntu, где уже запущен бэкенд GoGet. Выполняйте
их из репозитория. Установите конфигурацию до пакетов, чтобы при первом
старте системных служб Prometheus и Node Exporter слушали только loopback.

```bash
sudo install -d -m 0755 /etc/goget \
  /etc/systemd/system/prometheus.service.d \
  /etc/systemd/system/prometheus-node-exporter.service.d
sudo install -m 0644 deploy/monitoring/prometheus.yml /etc/goget/prometheus.yml
sudo install -m 0644 deploy/monitoring/prometheus.service.d/10-goget.conf \
  /etc/systemd/system/prometheus.service.d/
sudo install -m 0644 deploy/monitoring/prometheus-node-exporter.service.d/10-goget.conf \
  /etc/systemd/system/prometheus-node-exporter.service.d/
sudo systemctl daemon-reload
sudo apt-get install --no-install-recommends prometheus prometheus-node-exporter
promtool check config /etc/goget/prometheus.yml
sudo systemctl daemon-reload
sudo systemctl enable prometheus.service prometheus-node-exporter.service
sudo systemctl restart prometheus.service prometheus-node-exporter.service
```

Разверните версию бэкенда с `/metrics` по [инструкции](../backend/README.md).
Если порты бэкенда на сервере отличаются, обновите targets в
`/etc/goget/prometheus.yml`, проверьте файл через `promtool` и выполните
`sudo systemctl reload prometheus`.

## Проверка

С сервера после первого сбора метрик:

```bash
curl -fsS 'http://127.0.0.1:9090/-/ready'
curl -fsSG 'http://127.0.0.1:9090/api/v1/query' \
  --data-urlencode 'query=up' | jq '.data.result[] | {job: .metric.job, up: .value[1]}'
```

Для всех семи jobs ожидается `up=1`. Prometheus доступен на
`127.0.0.1:9090`. Для просмотра с другого компьютера используйте
SSH-туннель на этот порт. Node Exporter слушает `127.0.0.1:9100`.
Gateway отвечает `404` на публичный запрос `/metrics`.

## Grafana

Дашборд «GoGet — Мониторинг» загружается из репозитория при запуске
Grafana. Источник данных обращается к локальному Prometheus. В выпадающих
списках «Сервис», «Метод» и «Маршрут» можно отфильтровать HTTP-графики.

1. Разверните бэкенд с `goget_http_requests_total` и
   `goget_http_request_duration_seconds` по [инструкции](../backend/README.md).
2. Скопируйте конфигурацию привязки Grafana к loopback перед установкой пакета.

   ```bash
   sudo install -D -m 0644 deploy/monitoring/grafana-server.service.d/10-goget.conf \
     /etc/systemd/system/grafana-server.service.d/10-goget.conf
   sudo systemctl daemon-reload
   ```

3. Установите пакет Grafana OSS `grafana` из [официального APT-репозитория](https://grafana.com/docs/grafana/latest/setup-grafana/installation/debian/).
4. Из корня репозитория установите источник данных и дашборд.

   ```bash
   sudo install -D -m 0644 deploy/monitoring/grafana/provisioning/datasources/goget-prometheus.yml \
     /etc/grafana/provisioning/datasources/goget-prometheus.yml
   sudo install -D -m 0644 deploy/monitoring/grafana/provisioning/dashboards/goget.yml \
     /etc/grafana/provisioning/dashboards/goget.yml
   sudo install -D -m 0644 deploy/monitoring/grafana/dashboards/goget-monitoring.json \
     /var/lib/grafana/dashboards/goget/goget-monitoring.json
   sudo systemctl enable grafana-server
   sudo systemctl restart grafana-server
   curl -fsS http://127.0.0.1:3000/api/health
   ```

После запуска откройте SSH-туннель с рабочего компьютера. В своей обычной
команде подключения укажите `-N -L 127.0.0.1:13000:127.0.0.1:3000`.
Держите туннель открытым и перейдите в браузере на [http://127.0.0.1:13000](http://127.0.0.1:13000).
При новой установке Grafana попросит сменить начальный пароль администратора.
Откройте **Dashboards → GoGet → GoGet — Мониторинг**.

Верхние индикаторы показывают доступность целей Prometheus. HTTP-панели
показывают запросы в секунду, ответы 4xx и 5xx и p95 времени ответа.
Ресурсы сервера отображаются в процентах. Диск — корневая файловая система
`/`. Чтобы увидеть HTTP-графики после первой выкладки, сделайте несколько
запросов к нужному сервису и дождитесь как минимум двух сборов метрик.
