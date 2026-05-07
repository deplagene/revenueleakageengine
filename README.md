# Revenue Leakage Engine

Revenue Leakage Engine объясняет разницу между ожидаемой выручкой и фактически выставленной выручкой.
Система хранит коммерческие условия контракта, принимает billing facts, строит expected/actual revenue ledgers
и создает leakage cases для операционного расследования.

Проект отвечает на четыре вопроса:

- Что бизнес должен был заработать?
- Что он реально заработал, выставил или получил?
- Где возникло расхождение?
- Почему оно возникло на уровне процесса?

## Локальный запуск

Требования:

- Go версии из `go.mod`.
- Task CLI для выполнения команд из `Taskfile.yaml`.
- SQLite CLI для ручного seed-сценария.

Команды из `Taskfile.yaml` являются приоритетным способом работы с проектом:

```bash
task test
task test-integration
task test-kafka
task build
task run
task sqlc-generate
task proto:lint
task proto:gen
task ui-deps
task templ-generate
task templ-watch
task migrate-up
task compose:config
task compose:build
task compose:up
task compose:up-detached
task compose:down
task compose:logs
task compose:config-infisical
task compose:up-infisical
task compose:up-infisical-detached
```

`task run` открывает SQLite базу `./local.db`, применяет миграции
и поднимает HTTP API на `:8080` и gRPC API на `:9090`.
Операционный server-rendered UI доступен на `http://localhost:8080/ui`.

UI использует `templ` для HTML-компонентов и локальный `htmx` asset из
`internal/app/gateway/http/static/htmx.min.js`. Зависимости UI ставятся через
`task ui-deps`, генерация `*_templ.go` выполняется через `task templ-generate`.
Для локальной разработки UI можно использовать `task templ-watch`; proxy будет
доступен на `http://localhost:8081`, а приложение запускается через `task run`.
Миграции выполняются через `task migrate-up`; Taskfile сам установит `goose` в
`bin/goose`, если бинаря еще нет.

gRPC контракты лежат в `proto/revenueleakageengine/v1`, а сгенерированный Go-код
попадает в `internal/gen/proto/revenueleakageengine/v1`. Проверка и генерация
выполняются через `task proto:lint` и `task proto:gen`; Taskfile сам установит
`buf`, `protoc-gen-go` и `protoc-gen-go-grpc` в `bin/`.

Kafka/outbox pipeline в Sprint 7 проверяется unit-тестами через `task test-kafka`.
Сценарий такой: app use cases после успешной записи фактов или сверки кладут
JSON envelope в SQLite `outbox_events`; dispatcher читает pending rows и
публикует их в Kafka producer; Kafka consumer handler читает topics v1, проверяет
idempotency через `inbox_events` и вызывает ingestion/reconciliation app commands.
Доменные модели не зависят от Kafka-сообщений.

## Docker Compose

Локальный compose-стек поднимает сервис, Kafka в KRaft combined mode и Kafka UI:

- HTTP UI/API: `http://localhost:8080/ui`
- gRPC API: `localhost:9090`
- Kafka bootstrap для хоста: `localhost:9092`
- Kafka UI: `http://localhost:8082`

Проверка итоговой конфигурации при локальном `.env` или уже выставленных shell
variables:

```bash
task compose:config
```

Проверка итоговой конфигурации через Infisical Cloud UI:

```bash
INFISICAL_ENV=dev task compose:config-infisical
```

Запуск в foreground:

```bash
task compose:up
```

Запуск в фоне:

```bash
task compose:up-detached
```

Запуск в фоне через Infisical:

```bash
INFISICAL_ENV=dev task compose:up-infisical-detached
```

Compose создает топики приложения явно через `kafka-init`:

- `usage.records.v1`
- `billing.invoices.v1`
- `reconciliation.run.requested.v1`
- `reconciliation.run.completed.v1`
- `leakage.case.created.v1`

Переменные compose-запуска описаны в `.env.example`. Для Infisical эти же имена
нужно завести в Cloud UI выбранного окружения. Файл `.env` остается локальным и
не коммитится.

## Secret Manager

Сервис читает runtime-настройки из environment variables, поэтому Infisical будет
использоваться как внешний injector секретов, а не как зависимость domain/app
кода. Локальный путь без встраивания CLI в Docker image:

```bash
INFISICAL_ENV=dev INFISICAL_PROJECT_ID=<project-id> task compose:up-infisical
```

В этом режиме Infisical передает секреты процессу Docker Compose. Чтобы новый
секрет или runtime-параметр дошел до контейнера, его нужно явно добавить в
нужный блок `environment` или interpolation в `docker-compose.yaml`.

Для compose-стека Infisical является единым источником конфигурации: и секреты,
и несекретные runtime-параметры должны быть заведены в Cloud UI с теми же
именами, что указаны в `.env.example`. `docker-compose.yaml` не использует
локальные fallback defaults и завершится с ошибкой, если обязательной переменной
нет в Infisical или shell environment. Когда появятся production-секреты для
каждого сервиса, можно перейти на service-specific machine identity token в
compose-переменной `INFISICAL_TOKEN_REVENUELEAKAGEENGINE`.

Проверить, что Infisical отдает переменные:

```bash
INFISICAL_ENV=dev infisical run -- printenv | grep RLE_
INFISICAL_ENV=dev infisical run -- docker compose config
```

Если Task CLI недоступен, можно выполнить эквивалентные Go/Goose команды напрямую:

```bash
go test ./...
go test -tags=integration ./internal/app/gateway/http -run TestSprint4SmokeDatabaseDrivenReconciliation -count=1
go build ./...
go run ./cmd/revenueleakageengine
goose -dir internal/platform/sqlite/migrations sqlite3 ./local.db up
```

## Операционный UI

После `task run` откройте:

- `GET /ui` — dashboard с последними reconciliation runs, агрегатом leakage и очередью open cases по tenant filter.
- `GET /ui/reconciliation` — форма запуска database-driven reconciliation.
- `GET /ui/cases?tenant_id=<uuid>` — список leakage cases с htmx-фильтрами.
- `GET /ui/cases/{case_id}?tenant_id=<uuid>` — карточка кейса, evidence, root causes, status history и htmx-actions.
- `GET /ui/documents?tenant_id=<uuid>` — загрузка документов и запуск AI extraction draft для Sprint 8.

UI handlers остаются тонким transport layer: формы мапятся в существующие app/service commands,
а расчеты expected/actual revenue и lifecycle rules остаются в Go service/domain слоях.

## Document Intake & AI

Sprint 8 добавляет безопасный intake pipeline: оригинальные документы сохраняются
как immutable source, AI создает draft JSON, а core-сервисы `contract`,
`ingestion` и `reconciliation` получают данные только после будущего review/approve
шага. AI не пишет напрямую в таблицы контрактов, usage, invoices, ledger или
reconciliation.

Лимиты MVP:

- максимум 5 документов за один upload
- максимум 10 MiB на один файл
- хранилище документов: `/data/documents` в compose, `./data/documents` при прямом `go run`
- текущий NVIDIA extractor работает с текстовыми документами: TXT, CSV, JSON

Переменные:

```bash
RLE_AI_PROVIDER=nvidia
RLE_NVIDIA_API_KEY=<api-key>
RLE_NVIDIA_MODEL=nvidia/llama-3.1-nemotron-nano-8b-v1
RLE_DOCUMENT_STORAGE_PATH=/data/documents
RLE_DOCUMENT_MAX_FILES_PER_UPLOAD=5
RLE_DOCUMENT_MAX_FILE_BYTES=10485760
```

Upload через API:

```bash
curl -sS -X POST http://localhost:8080/api/v1/documents \
  -F tenant_id=11111111-1111-1111-1111-111111111111 \
  -F source_type=mixed \
  -F documents=@./examples/invoice.txt
```

Запуск extraction draft:

```bash
curl -sS -X POST http://localhost:8080/api/v1/documents/<document_id>/extract \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "draft_type": "mixed_facts"
  }'
```

Model plan:

- `nvidia/llama-3.1-nemotron-nano-8b-v1` — текстовый extractor MVP.
- `nvidia/llama-3.1-nemotron-nano-vl-8b-v1` — следующий шаг для OCR/image/PDF сценариев.
- `nvidia/llama-3.3-nemotron-super-49b-v1.5` — опциональный валидатор draft JSON.

## Money convention

Все денежные суммы передаются и хранятся в minor units.
Например, `500000` для `USD` означает `$5,000.00`, а `1` означает `$0.01`.

## End-to-End smoke сценарий

Этот сценарий воспроизводит Sprint 1-3: контракт и тарифы, ingestion usage/invoice фактов,
запуск reconciliation и создание leakage case.

### 1. Запустите сервер

```bash
task run
```

### 2. Создайте tenant и customer

Публичных endpoints для tenant/customer пока нет, поэтому локальный smoke-сценарий seed-ит эти reference rows напрямую.

```bash
sqlite3 ./local.db <<'SQL'
INSERT OR IGNORE INTO tenants (id, name, currency, timezone, created_at)
VALUES (
  '11111111-1111-1111-1111-111111111111',
  'Acme',
  'USD',
  'UTC',
  '2026-04-01T00:00:00Z'
);

INSERT OR IGNORE INTO customer_accounts (
  id,
  tenant_id,
  external_id,
  name,
  status,
  created_at
) VALUES (
  '22222222-2222-2222-2222-222222222222',
  '11111111-1111-1111-1111-111111111111',
  'acme-customer',
  'Acme Account',
  'active',
  '2026-04-01T00:00:00Z'
);
SQL
```

### 3. Создайте billable item

```bash
curl -sS -X POST http://localhost:8080/api/v1/contracts/billable-items \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "44444444-4444-4444-4444-444444444444",
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "code": "platform_subscription",
    "name": "Platform subscription",
    "category": "platform",
    "unit": "events",
    "pricing_mode": "usage",
    "status": "active"
  }'
```

### 4. Создайте contract

```bash
curl -sS -X POST http://localhost:8080/api/v1/contracts \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "33333333-3333-3333-3333-333333333333",
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "customer_id": "22222222-2222-2222-2222-222222222222",
    "external_id": "contract-acme-2026",
    "status": "active",
    "start_date": "2026-04-01T00:00:00Z",
    "currency": "USD",
    "version": 1,
    "billing_model": "usage_based"
  }'
```

### 5. Добавьте fixed fee term

```bash
curl -sS -X POST http://localhost:8080/api/v1/contracts/33333333-3333-3333-3333-333333333333/terms \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "fixed_fee",
    "code": "platform_subscription",
    "amount": 500000,
    "currency": "USD",
    "effective_from": "2026-04-01T00:00:00Z",
    "priority": 1
  }'
```

### 6. Добавьте usage rate term

```bash
curl -sS -X POST http://localhost:8080/api/v1/contracts/33333333-3333-3333-3333-333333333333/terms \
  -H 'Content-Type: application/json' \
  -d '{
    "type": "usage_rate",
    "code": "platform_subscription",
    "amount": 1,
    "currency": "USD",
    "expression": {
      "included_quantity": 100000,
      "unit": "events"
    },
    "effective_from": "2026-04-01T00:00:00Z",
    "priority": 2
  }'
```

### 7. Загрузите usage records

```bash
curl -sS -X POST http://localhost:8080/api/v1/ingest/usage \
  -H 'Content-Type: application/json' \
  -d '{
    "records": [
      {
        "tenant_id": "11111111-1111-1111-1111-111111111111",
        "customer_id": "22222222-2222-2222-2222-222222222222",
        "contract_id": "33333333-3333-3333-3333-333333333333",
        "billable_item_id": "44444444-4444-4444-4444-444444444444",
        "external_id": "usage-april-001",
        "usage_time": "2026-04-10T12:00:00Z",
        "quantity": 180000,
        "unit": "events",
        "source_system": "metering",
        "trace_id": "trace-sprint-4"
      }
    ]
  }'
```

### 8. Загрузите invoice

```bash
curl -sS -X POST http://localhost:8080/api/v1/ingest/invoices \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "customer_id": "22222222-2222-2222-2222-222222222222",
    "contract_id": "33333333-3333-3333-3333-333333333333",
    "external_id": "stripe-invoice-april",
    "number": "INV-2026-04",
    "period_start": "2026-04-01T00:00:00Z",
    "period_end": "2026-05-01T00:00:00Z",
    "issued_at": "2026-05-01T09:00:00Z",
    "due_at": "2026-05-15T00:00:00Z",
    "currency": "USD",
    "total_amount_minor_units": 464000,
    "status": "issued",
    "source_system": "stripe",
    "lines": [
      {
        "billable_item_id": "44444444-4444-4444-4444-444444444444",
        "description": "Platform subscription and usage",
        "quantity": 1,
        "unit_price_minor_units": 464000,
        "discount_amount_minor_units": 0,
        "tax_amount_minor_units": 0,
        "line_total_minor_units": 464000,
        "source_ref": "stripe:line-april-001"
      }
    ]
  }'
```

### 9. Запустите reconciliation

```bash
curl -sS -X POST http://localhost:8080/api/v1/reconciliation/run \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "11111111-1111-1111-1111-111111111111",
    "contract_id": "33333333-3333-3333-3333-333333333333",
    "currency": "USD",
    "trace_id": "trace-sprint-4",
    "minimum_leakage_minor_units": 0,
    "period": {
      "start": "2026-04-01T00:00:00Z",
      "end": "2026-05-01T00:00:00Z"
    }
  }'
```

Expected revenue считается как `500000 + ((180000 - 100000) * 1) = 580000`.
Actual revenue приходит из invoice line на `464000`.
Reconciliation должен создать underbilling case на `116000`.

### 10. Проверьте cases

```bash
curl -sS 'http://localhost:8080/api/v1/cases?tenant_id=11111111-1111-1111-1111-111111111111'
```

В ответе должен быть один case со статусом `open`, типом `underbilling` и заполненным `reconciliation_run_id`.

## Автоматический smoke-test

Интеграционный тест выполняет тот же сценарий через in-memory SQLite и HTTP router:

```bash
task test-integration
```
