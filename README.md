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
task build
task run
task sqlc-generate
task proto:lint
task proto:gen
task ui-deps
task templ-generate
task templ-watch
task migrate-up
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

UI handlers остаются тонким transport layer: формы мапятся в существующие app/service commands,
а расчеты expected/actual revenue и lifecycle rules остаются в Go service/domain слоях.

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
