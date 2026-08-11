# Tappix: руководство для совместной разработки

Этот документ позволяет новому разработчику получить проект, запустить весь стек, войти во все интерфейсы и безопасно начать работу без устных инструкций.

## 1. Что такое Tappix

Tappix — multi-tenant SaaS-платформа лояльности для малого и среднего бизнеса. Одна установка обслуживает несколько компаний, но данные каждой компании изолированы по `company_id`.

Основные пользовательские роли:

- Platform Owner управляет компаниями, подписками и системными настройками.
- Владелец бизнеса настраивает программу лояльности, филиалы, сотрудников, кампании, аналитику и интеграции.
- Сотрудник работает в упрощённом Staff Mode: находит клиента, регистрирует покупку и проводит бонусную операцию.
- Клиент использует цифровую карту, QR-код, бонусы, награды, историю и настройки согласий.

Основные реализованные модули:

- CRM клиентов, посещения и бонусный ledger;
- филиалы, сотрудники, NFC и QR;
- клиентская карта и гостевой портал;
- Guided Onboarding для запуска программы;
- программы лояльности и награды;
- кампании, holdout, ROI и триггерные автоматизации;
- реферальная программа и партнёрский кросс-маркетинг;
- бизнес-аналитика, retention cohorts, RFM, LTV и churn risk;
- FIFO bonus lots и bonus liability;
- Integration Hub и канонический API чеков;
- Poster import, webhook и reconciliation;
- сайт бизнеса, запись, отзывы, файлы и уведомления;
- тарифы, права, аудит и Platform Admin.

## 2. Репозиторий и рабочий процесс

Репозиторий:

```text
https://github.com/fdfgdffgfdfgd-sudo/tappixloyal
```

Рекомендуемый процесс работы:

1. Не работать напрямую в `main`.
2. Перед новой задачей обновить `main`.
3. Создать отдельную ветку с понятным названием.
4. Делать небольшие тематические коммиты.
5. Перед push запускать проверки.
6. Открыть Pull Request в `main`.
7. После зелёного CI выполнить review и merge.

Пример:

```bash
git clone https://github.com/fdfgdffgfdfgd-sudo/tappixloyal.git
cd tappixloyal
git switch main
git pull --ff-only
git switch -c feature/customer-search
```

После завершения задачи:

```bash
git status
git add path/to/changed-file
git commit -m "Add customer server search"
git push -u origin feature/customer-search
```

Не используйте `git add -A`, если в рабочем дереве есть чужие или экспериментальные изменения. Сначала проверьте `git status` и `git diff`.

## 3. Что установить

Обязательно:

- Git;
- Docker Desktop или Docker Engine с Compose v2;
- Node.js 22;
- npm 10 или новее.

Go 1.25 устанавливать необязательно: API собирается и тестируется через Docker. Локальный Go полезен только для более быстрой backend-разработки.

Проверка окружения:

```bash
git --version
docker --version
docker compose version
node --version
npm --version
```

## 4. Первый запуск

В корне репозитория:

```bash
npm ci
docker compose up -d --build
npm run db:migrate
docker compose ps
```

Все сервисы в `docker compose ps` должны иметь статус `Up`, а API и frontend — `healthy`.

Проверка API:

```bash
curl http://localhost:8080/health
```

Ожидаемый результат содержит:

```json
{"success":true,"data":{"checks":{"postgres":"ok","redis":"ok"},"status":"ok"}}
```

Повторный запуск:

```bash
npm run stack:up
npm run db:migrate
```

Остановка:

```bash
npm run stack:down
```

Обычный `docker compose down` сохраняет базу в Docker volume. Не добавляйте `-v`, если не хотите удалить локальные данные.

## 5. Адреса локальных сервисов

| Сервис | Адрес | Назначение |
|---|---|---|
| Tappix | `http://localhost:8088` | основной интерфейс через Nginx |
| Business Login | `http://localhost:8088/login` | вход владельца бизнеса |
| Guided Onboarding | `http://localhost:8088/onboarding` | мастер запуска программы |
| Customer Portal | `http://localhost:8088/customer` | клиентская карта и вход гостя |
| Platform Admin | `http://localhost:8088/admin` | системное управление |
| API | `http://localhost:8080/api/v1` | REST API |
| API Health | `http://localhost:8080/health` | PostgreSQL/Redis health check |
| Mailpit | `http://localhost:8025` | перехват тестовых email |
| PostgreSQL | `localhost:5432` | локальная база |
| Redis | `localhost:6379` | сессии, rate limits и очереди |

## 6. Тестовые аккаунты

Аккаунты предназначены только для локальной разработки.

### Владелец Dentline

```text
Email: armat@tappix.kz
Пароль: Tappix2026!
```

### Владелец DocMed

```text
Email: owner@docmed.kz
Пароль: DocMed2026!
```

DocMed используется в integration-тестах для проверки tenant isolation.

### Platform Owner

```text
Email: admin@tappix.kz
Пароль: Admin2026!
```

### Клиент Dentline

```text
Компания: dentline
Телефон: +7 700 333 33 33
PIN: 1234
```

## 7. Архитектура

```text
Browser
  │
  ▼
Nginx :8088
  ├── Next.js web :3000
  └── Go REST API :8080
        ├── PostgreSQL :5432
        ├── Redis :6379
        ├── Mailpit/SMTP :1025
        └── background workers
              ├── analytics projections
              ├── integrations and reconciliation
              ├── campaign automations
              └── outbound webhooks
```

Технологии:

- frontend: Next.js 16, React 19, TypeScript, CSS, Lucide icons;
- backend: Go 1.25, `net/http`, pgx;
- data: PostgreSQL 17 и JSONB;
- runtime state: Redis 7;
- локальная почта: Mailpit;
- reverse proxy: Nginx;
- инфраструктура разработки: Docker Compose;
- CI: GitHub Actions.

## 8. Структура каталогов

```text
tappix/
├── apps/
│   ├── api/
│   │   ├── cmd/server/          # запуск Go API
│   │   ├── internal/httpapi/    # handlers, auth, workers
│   │   ├── internal/integration/# POSAdapter и Poster
│   │   └── migrations/          # SQL up/down migrations
│   └── business/
│       ├── app/                 # Next.js App Router и CSS
│       ├── components/          # страницы и UI-компоненты
│       └── lib/                 # API client и общие функции
├── docs/                        # архитектура и продуктовые решения
├── infrastructure/
│   ├── nginx/                   # reverse proxy
│   └── scripts/                 # migrations, tests, backup
├── docker-compose.yml           # полный локальный стек
├── package.json                 # общие команды
└── .env.example                 # шаблон переменных окружения
```

## 9. Frontend

Frontend находится в `apps/business` и использует Next.js App Router.

Основные маршруты:

- `/` — обзор бизнеса;
- `/customers` — клиенты;
- `/scanner` — Staff Mode;
- `/loyalty` — программа лояльности;
- `/campaigns` — кампании и автоматизации;
- `/referrals` — рефералы;
- `/analytics` — аналитика;
- `/integrations` — POS Integration Hub;
- `/devices` — NFC/QR;
- `/onboarding` — Guided Onboarding;
- `/customer` — клиентская карта;
- `/admin` — Platform Admin.

Запуск только frontend в dev-режиме:

```bash
npm run dev
```

Frontend откроется на `http://localhost:3000`, а API должен быть доступен отдельно.

При разработке интерфейса:

- сохраняйте существующую дизайн-систему и CSS tokens;
- проверяйте ширину 375 px и desktop;
- используйте семантические элементы и `focus-visible`;
- не скрывайте ошибки API;
- предусматривайте loading, empty, error и success states;
- не загружайте большие списки клиентов в select — используйте серверный поиск.

## 10. Backend и API

Точка запуска API:

```text
apps/api/cmd/server/main.go
```

Регистрация маршрутов:

```text
apps/api/internal/httpapi/api.go
```

API использует envelope:

```json
{
  "success": true,
  "data": {}
}
```

Ошибка:

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Описание ошибки"
  }
}
```

Основная документация API находится в `docs/openapi.yaml`.

При добавлении endpoint необходимо:

1. Добавить handler.
2. Зарегистрировать маршрут в `api.go`.
3. Установить роль или permission.
4. Проверить tenant принадлежность всех входных ID.
5. Добавить validation и стабильный error code.
6. Обновить OpenAPI.
7. Добавить unit или integration test.

## 11. Multi-tenancy и безопасность

Ключевое правило: `company_id` нельзя принимать как доверенный tenant из body или query.

Tenant определяется из проверенного access token или API key. Все бизнес-запросы должны ограничиваться `company_id`.

Защита состоит из нескольких уровней:

- JWT и refresh sessions в Redis;
- роли и таблица `role_permissions`;
- тарифные entitlements и server-side limits;
- tenant-фильтры в SQL;
- PostgreSQL Row Level Security;
- audit log;
- API key scopes;
- idempotency keys;
- HMAC для webhooks.

При работе с филиалом, клиентом, сотрудником, подключением или чеком обязательно проверяйте, что все связанные сущности принадлежат одному tenant.

Не добавляйте секреты и реальные токены в Git. Используйте `.env`.

## 12. Данные и основные сущности

```text
Клиент
├── покупки / чеки
│   ├── позиции
│   ├── оплаты
│   ├── начисления и списания
│   └── источник / кампания
├── события
├── коммуникации
└── рефералы
```

Основные таблицы:

- `customers`, `visits`, `bonus_ledger`;
- `sales_transactions`, `sales_transaction_items`, `payments`;
- `customer_events`, `customer_attributions`;
- `integration_connections`, `integration_jobs`, `integration_failures`;
- `campaign_conversions`, `campaign_automations`;
- `referral_programs`, `referral_attributions`, `referral_rewards`;
- `bonus_lots`, `bonus_lot_redemptions`;
- `analytics_daily_facts`, `analytics_customer_features`;
- `report_schedules`.

Чеки защищены от повторной доставки POS ограничением:

```text
UNIQUE(company_id, provider, external_id)
```

## 13. Миграции базы

Миграции находятся в `apps/api/migrations` и применяются по порядку.

Формат:

```text
000028_short_description.up.sql
000028_short_description.down.sql
```

Применение:

```bash
npm run db:migrate
```

Правила:

- не изменяйте уже применённую production-миграцию;
- для нового изменения создавайте следующий номер;
- используйте `ON CONFLICT`, `IF EXISTS` и `IF NOT EXISTS`, когда это делает повтор безопасным;
- добавляйте индексы для tenant/time и внешних идентификаторов;
- добавляйте RLS policy для tenant-таблиц;
- проверяйте миграцию на чистой базе и на существующей схеме;
- destructive down migration должна быть очевидной и осознанной.

## 14. Background workers

Workers запускаются только после готовности схемы.

Сейчас они выполняют:

- заполнение аналитических проекций;
- обработку integration jobs;
- Poster synchronization и reconciliation;
- доставку webhooks;
- campaign automations;
- реферальные награды и финансовые фоновые операции.

Требования к worker-задачам:

- идемпотентность;
- ограниченное число повторов;
- `failed`/dead-letter состояние;
- сохранение `last_error`;
- блокировка `FOR UPDATE SKIP LOCKED`, если задача забирается конкурентно;
- безопасный повтор оператором;
- tenant context в каждой операции.

## 15. Проверки перед Pull Request

API tests:

```bash
npm run test:api
```

Frontend lint:

```bash
npm run lint
```

Production build и TypeScript:

```bash
npm run build
```

Integration tests требуют запущенный стек и миграции:

```bash
npm run stack:up
npm run db:migrate
npm run test:integration
```

Полная проверка:

```bash
npm test
```

Проверка форматирования Go без локальной установки Go:

```bash
docker run --rm -v "$PWD/apps/api:/src" -w /src golang:1.25-alpine sh -c 'test -z "$(gofmt -l .)"'
```

GitHub Actions запускает `quality` и `integration` для push и Pull Request.

## 16. Переменные окружения

Для локального запуска значения по умолчанию заданы в Compose. Для собственной конфигурации:

```bash
cp .env.example .env
```

Главные переменные:

| Переменная | Назначение |
|---|---|
| `POSTGRES_DB` | имя базы |
| `POSTGRES_USER` | пользователь PostgreSQL |
| `POSTGRES_PASSWORD` | пароль PostgreSQL |
| `JWT_SECRET` | подпись access tokens |
| `REDIS_ADDR` | адрес Redis |
| `APP_URL` | публичный адрес приложения |
| `TAPPIX_PORT` | внешний порт Nginx |
| `API_PORT` | внешний порт API |
| `SMTP_HOST`, `SMTP_PORT` | отправка email |
| `WHATSAPP_ACCESS_TOKEN` | Meta WhatsApp Cloud API |
| `WHATSAPP_PHONE_NUMBER_ID` | WhatsApp sender ID |
| `OTP_DEV_MODE` | выдача dev OTP без реальной доставки |

В production обязательно замените `POSTGRES_PASSWORD`, `JWT_SECRET`, SMTP/WhatsApp credentials и `APP_URL`. Установите `OTP_DEV_MODE=false`.

## 17. Backup и восстановление

Создание резервной копии:

```bash
npm run db:backup
```

Файлы сохраняются в `backups/` и не попадают в Git.

Восстановление требует явного подтверждения:

```bash
CONFIRM_RESTORE=tappix sh infrastructure/scripts/restore-postgres.sh /absolute/path/to/backup.sql.gz
```

Восстановление перезаписывает данные базы. Никогда не запускайте его без проверки точного файла и окружения.

## 18. Частые проблемы

### Порт уже занят

Измените порты в `.env`:

```text
TAPPIX_PORT=8090
API_PORT=8091
```

### Изменения frontend не появились

Production frontend работает из Docker image:

```bash
docker compose build web
docker compose up -d web nginx
```

### Изменения API не появились

```bash
docker compose build api
docker compose up -d api
```

### API сообщает об отсутствующей таблице

```bash
npm run db:migrate
docker compose restart api
```

### Посмотреть логи

```bash
docker compose logs -f api
docker compose logs -f web
docker compose logs -f postgres
```

### Полностью чистый локальный запуск

Команда ниже удаляет локальную базу и uploads. Использовать только если данные не нужны:

```bash
docker compose down -v
docker compose up -d --build
npm run db:migrate
```

## 19. Где читать подробнее

- `README.md` — быстрый запуск;
- `docs/ARCHITECTURE.md` — базовые архитектурные решения;
- `docs/openapi.yaml` — контракт API;
- `docs/business-data-foundation.md` — бизнес-данные и аналитика;
- `docs/integration-core.md` — POS integration core;
- `docs/campaign-automations-partnerships.md` — кампании и партнёрства;
- `docs/platform-blueprint-v3.md` — продуктовый blueprint;
- `design-system/tappix/MASTER.md` — визуальная система.

## 20. Definition of Done

Задача считается завершённой, когда:

- реализован рабочий сценарий, а не только интерфейс;
- сохранена tenant isolation;
- обработаны loading, empty, error и success states;
- добавлены validation и понятные ошибки;
- обновлены миграции и OpenAPI, если менялся backend;
- добавлены или обновлены тесты;
- `npm run test:api`, `npm run lint`, `npm run build` проходят;
- integration test проходит для затронутого end-to-end сценария;
- в коммит не попали `.env`, токены, backups и чужие изменения;
- Pull Request получил зелёный CI.

Если новый участник прошёл разделы 3–6 и увидел dashboard, клиентскую карту и Platform Admin, окружение готово к совместной работе.
