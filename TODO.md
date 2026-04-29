## Спринт 1: Контракты как «Источник Правды»

**Цель:** Научить систему хранить и отдавать условия сделок (Fixed Fee, Usage Rates).

- **Реализация `internal/service/contract`**:
  - Создание Store (SQLite) для таблиц `contracts`, `billable_items`, `contract_terms`.
  - Логика `GetEffectiveTerms(contractID, timestamp)` — получение цен, действующих на конкретный момент времени.
- **API в `internal/app/gateway/http`**:
  - `POST /contracts` — создание контракта с привязкой к `tenant_id`.
  - `POST /contracts/{id}/terms` — добавление условий, например "Абонентская плата $500/мес".
- **Пример**:
  ```json
  {
    "type": "fixed_fee",
    "code": "platform_subscription",
    "amount": 50000,
    "currency": "USD",
    "effective_from": "2024-01-01T00:00:00Z"
  }
  ```

---

## Спринт 2: Ингерстия фактов (Usage & Invoices)

**Цель:** Перестать зависеть от разовых выгрузок в запросе и начать накапливать данные в БД.

- **Реализация `internal/app/ingestion`**:
  - Сервис приема сырых данных: `UsageRecords` (потребление ресурсов) и `Invoices` (то, что биллинг уже выставил).
  - Идемпотентность через `external_id`, чтобы повторный POST одного и того же факта не дублировал запись.
- **API**:
  - `POST /ingest/usage` — пакетная загрузка логов потребления.
  - `POST /ingest/invoices` — загрузка выставленных счетов для сверки.
- **Пример**:
  - Загружаем 1000 записей о трафике за март. Система складывает их в `usage_records`, привязывая к `tenant_id`.

---

## Спринт 3: Сверка по данным из БД (Database-Driven)

**Цель:** Запуск процесса реконсиляции по идентификаторам, а не по дампу данных.

- **Переработка `internal/app/reconciliation/workflow.go`**:
  - Вход: `tenant_id`, `contract_id`, `period`.
  - Workflow сам идет в `service/contract` за ценами и в `service/ingestion` за фактами потребления и инвойсами.
- **Связь с кейсами**:
  - При обнаружении утечки создается `leakage_case`, в который записывается `reconciliation_run_id`.
  - В базе данных фиксируется сам запуск (`reconciliation_runs`) со статистикой: сколько строк обработано, сколько денег утекло.
- **Пример**:
  - Запрос: `POST /reconciliation/run { "tenant_id": "T1", "contract_id": "C1", "period": { ... } }`.
  - Результат: система находит контракт, видит в БД 500GB трафика, считает ожидаемую сумму $1025, сравнивает с инвойсом на $1000 и создает Case #42 на $25.

---

## Спринт 4: End-to-End сценарий и документация API

**Цель:** Зафиксировать рабочий путь Sprint 1-3 как воспроизводимый сценарий: контракт -> условия -> usage/invoice факты -> reconciliation run -> leakage case.

- **Интеграционный smoke-тест**:
  - Создать tenant/customer/contract/billable item.
  - Добавить `fixed_fee` и `usage_rate` terms.
  - Загрузить usage records и invoice через ingestion API/app слой.
  - Запустить `POST /reconciliation/run`.
  - Проверить записи в `expected_revenue_entries`, `actual_revenue_entries`, `reconciliation_runs`, `leakage_cases`.
- **Документация в `README.md`**:
  - Описать минимальный локальный запуск: миграции, сервер, примеры запросов.
  - Добавить JSON payload для `contracts`, `billable-items`, `terms`, `ingest/usage`, `ingest/invoices`, `reconciliation/run`.
  - Описать money convention: все суммы передаются в minor units.
- **Критерий готовности**:
  - Новый разработчик может поднять проект локально и воспроизвести leakage case по README.

---

## Спринт 5: Операционный UI на htmx + templ

**Цель:** Сделать простой server-rendered интерфейс для оператора, без SPA и без дублирования бизнес-логики на фронте.

- **Технологии**:
  - `templ` для типизированных HTML-компонентов.
  - `htmx` для частичных обновлений: запуск сверки, фильтры, смена статуса кейса.
  - Стандартные HTTP handlers остаются в `internal/app/gateway/http`; UI handlers мапят формы в app commands.
- **Экраны**:
  - Dashboard: последние reconciliation runs и агрегаты по leakage amount.
  - Run reconciliation form: `tenant_id`, `contract_id`, period, threshold.
  - Runs list/detail: статус, counts, leakage amount, trace id.
  - Cases list/detail: severity, status, evidence, root cause, assignee.
  - Case actions: investigate, resolve, dismiss, assign.
- **Границы**:
  - Это внутренний операторский/разработческий интерфейс, а не клиентский портал.
  - На этом этапе допустимы технические поля вроде `tenant_id`, `contract_id` и ручной запуск сверки.
  - Клиентский сценарий загрузки документов выносится в отдельный sprint, чтобы не смешивать UI консоли и intake pipeline.
  - View-компоненты не содержат money/reconciliation logic.
  - htmx не вызывает domain/service напрямую; только HTTP endpoints.
  - UI использует существующие app/service use cases.

---

## Спринт 6: gRPC API для machine-to-machine интеграций

**Цель:** Добавить стабильный внутренний API для сервисов, которым нужен типизированный контракт вместо HTTP/JSON.

- **Proto contracts**:
  - `ReconciliationService.Run`
  - `ReconciliationService.GetRun`
  - `ReconciliationService.ListCases`
  - Позже: `IngestionService.PushUsage`, `IngestionService.PushInvoice`.
- **Реализация**:
  - `.proto` хранить отдельно от домена, generated code держать в `internal/gen/proto`.
  - gRPC handlers размещать в `internal/app/gateway/grpc`.
  - DTO/proto mapping держать на gateway boundary.
- **Границы**:
  - Domain entities не зависят от protobuf.
  - gRPC handlers только валидируют transport-level ввод и вызывают app use cases.
  - Ошибки мапятся в gRPC status codes явно.

---

## Спринт 7: Kafka, outbox и асинхронные события

**Цель:** Подготовить систему к потоковой ingestion и event-driven интеграциям без потери аудита и идемпотентности.

- **Topics v1**:
  - `usage.records.v1`
  - `billing.invoices.v1`
  - `reconciliation.run.requested.v1`
  - `reconciliation.run.completed.v1`
  - `leakage.case.created.v1`
- **Outbox/inbox**:
  - Публиковать domain/app events через outbox после успешной транзакции.
  - Consumer должен быть идемпотентным по `event_id` или source `external_id`.
  - Partition key: `tenant_id` или `contract_id` в зависимости от ordering requirement.
- **Consumers**:
  - Usage consumer мапит Kafka message в ingestion app command.
  - Invoice consumer мапит Kafka message в ingestion app command.
  - Reconciliation requested consumer запускает workflow по `tenant_id`, `contract_id`, `period`.
- **Границы**:
  - Kafka не заменяет service/app слой.
  - Message schemas не протекают в `internal/domain`.
  - Повторная доставка события не должна создавать дубли usage, invoices, runs или cases.

---

## Спринт 8: Document Intake & AI Extraction Pipeline

**Цель:** Построить безопасный pipeline загрузки документов, в котором клиент или оператор загружает исходные файлы, ИИ извлекает структурированные факты, а core-сервис получает проверенный JSON для существующих use cases.

- **Сценарий взаимодействия**:
  - Клиент загружает договоры, счета, usage exports или billing exports через будущий client portal/API.
  - Система сохраняет оригинальный документ как immutable source: `document_id`, `tenant_id`, `source_type`, `file_hash`, `uploaded_at`.
  - AI/OCR/parser извлекает draft facts: contract terms, billable items, usage records, invoices.
  - Результат сохраняется как structured JSON draft с confidence score и ссылками на evidence: page, row, field, document_id.
  - JSON проходит schema validation, business validation и idempotency checks.
  - Оператор подтверждает, правит или отклоняет draft.
  - Только approved draft отправляется в существующие сервисы: `contract`, `ingestion`, `reconciliation`.
- **JSON-контракты**:
  - `contract_terms_draft`: условия договора, pricing model, effective period, currency, source refs.
  - `usage_records_draft`: потребление за период, external id, quantity, unit, source refs.
  - `invoices_draft`: выставленные суммы, invoice number, line items, billing period, source refs.
  - Draft JSON не является domain entity; это transport/input model на boundary document intake.
- **Архитектура**:
  - `internal/app/document`: use cases `UploadDocument`, `ExtractDocumentFacts`, `ReviewExtraction`, `ApproveExtraction`, `RejectExtraction`.
  - `internal/service/document`: validation policy, confidence policy, evidence mapping, duplicate detection.
  - `internal/platform/ai`: provider adapter для LLM/OCR extraction.
  - `internal/platform/storage`: локальное файловое хранилище для MVP, позже S3-compatible storage.
  - Существующие сервисы остаются владельцами бизнес-данных; document intake только готовит проверенный вход.
- **Границы**:
  - ИИ не пишет напрямую в таблицы контрактов, usage, invoices, ledger или reconciliation.
  - ИИ не принимает финансовые решения и не считает expected/actual revenue.
  - Любой extracted fact должен иметь source reference на документ, страницу, строку или поле.
  - При низкой уверенности draft требует ручного подтверждения.
  - Оригинальные документы и extraction output версионируются для аудита.
- **Критерий готовности**:
  - Можно загрузить документ или экспорт и получить draft JSON по утвержденной schema.
  - Draft можно подтвердить и отправить в существующие ingestion/contract use cases.
  - В UI видно, какие поля извлечены ИИ, какие подтверждены оператором и откуда взят каждый факт.

---

## Спринт 9: AI-assisted Revenue Investigation

**Цель:** Добавить ИИ-помощника для объяснения и расследования leakage cases без замены детерминированной финансовой логики.

- **Роль ИИ**:
  - Объяснять уже найденный leakage case простым языком.
  - Предлагать вероятные root causes на основе evidence, contract terms, usage records, invoices и ledger entries.
  - Использовать подтвержденные факты и source references из document intake pipeline, если кейс был создан на основе загруженных документов.
  - Формировать next actions для оператора: что проверить, у кого запросить данные, какие поля выглядят подозрительно.
  - Отвечать на вопросы по кейсу в режиме "chat with case", используя только контекст из БД.
- **Жесткие границы**:
  - ИИ не считает `expected_revenue` и `actual_revenue`.
  - ИИ не пишет в `expected_revenue_entries`, `actual_revenue_entries`, `reconciliation_runs`.
  - ИИ не закрывает, не резолвит и не dismiss-ит кейсы автоматически.
  - ИИ не создает root cause как факт; он создает suggestion, который должен подтвердить оператор.
  - Каждый AI output должен ссылаться на evidence IDs, contract term IDs, invoice IDs или ledger entry IDs.
  - Если данных недостаточно, ИИ должен явно вернуть "недостаточно данных", а не достраивать выводы.
- **Архитектура**:
  - `internal/app/ai`: use cases `GenerateCaseInsight`, `AskCaseQuestion`, `AcceptSuggestion`, `RejectSuggestion`.
  - `internal/service/ai`: сбор контекста, prompt policy, schema validation, safety rules.
  - `internal/platform/ai`: provider adapter для конкретной LLM.
  - `internal/domain/leakage` хранит только подтвержденные оператором выводы, а не сырые AI догадки.
- **Хранение**:
  - Добавить таблицу `ai_insights`.
  - Поля: `id`, `case_id`, `reconciliation_run_id`, `kind`, `status`, `model`, `prompt_version`, `input_hash`, `output_json`, `created_at`, `accepted_by`, `accepted_at`.
  - `output_json` должен быть структурированным: summary, probable causes, evidence refs, next actions, confidence.
- **Поток**:
  - Reconciliation создает `leakage_case`.
  - Outbox публикует `leakage.case.created.v1`.
  - AI worker собирает case context из БД.
  - LLM возвращает structured JSON по заранее заданной schema.
  - UI показывает AI insight оператору.
  - Оператор принимает, отклоняет или комментирует suggestion.
- **Критерий готовности**:
  - Для leakage case можно получить AI summary и next actions.
  - Все AI ответы сохраняются с prompt version и input hash.
  - В UI видно, какие выводы являются AI suggestion, а какие подтверждены оператором.

---
