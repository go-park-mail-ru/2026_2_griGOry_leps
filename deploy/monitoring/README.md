# Мониторинг GoGet

Prometheus каждые 15 секунд опрашивает `/metrics` трёх сервисов, Node
Exporter и собственный endpoint. Сервисы отдают стандартные метрики Go
и процесса, Node Exporter — метрики хоста. Имена jobs в Prometheus:
`auth`, `ads`, `search`, `node`, `prometheus`.

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

Для всех пяти jobs ожидается `up=1`. Prometheus доступен на
`127.0.0.1:9090`. Для просмотра с другого компьютера используйте
SSH-туннель на этот порт. Node Exporter слушает `127.0.0.1:9100`.
Gateway отвечает `404` на публичный запрос `/metrics`.

## Запросы для дашборда GOS-49

| Панель | PromQL |
| --- | --- |
| CPU, % | `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{job="node",mode="idle"}[5m])))` |
| Используемая RAM, % | `100 * (1 - node_memory_MemAvailable_bytes{job="node"} / node_memory_MemTotal_bytes{job="node"})` |
| Корневой диск, % | `100 * (1 - node_filesystem_avail_bytes{job="node",mountpoint="/"} / node_filesystem_size_bytes{job="node",mountpoint="/"})` |
| Доступность сервисов | `up{job=~"auth\|ads\|search"}` |

Запросы, ошибки HTTP и длительность ответов добавляются задачей GOS-48
в реестр Prometheus каждого сервиса.
