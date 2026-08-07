# Tappix Platform Blueprint v3

Статус: архитектура для утверждения. Массовая реализация начинается только после подтверждения Армата.

## 0. Аудит текущего проекта

### Что сохраняем

- Go HTTP API, PostgreSQL 17, Redis, Docker Compose и nginx.
- JWT access/refresh, отзыв сессий, password reset и rate limiting.
- `company_id` как текущий tenant key и PostgreSQL RLS как второй защитный слой.
- Сущности компаний, пользователей, memberships, клиентов, визитов, ledger, устройств, подписок, модулей, наград и аудита.
- Рабочие NFC/QR device tokens, публичную регистрацию, customer OTP и Guest API.
- Текущие данные Dentline и других tenants — мигрируются, а не пересоздаются.

### Что перерабатываем

- `companies` становится совместимым alias для домена Tenant; физическое переименование не требуется в первой миграции.
- Простые `role`-проверки заменяются permission policy + membership + entitlement + limit checks.
- `modules/company_modules` становятся read-only для Business Owner; управляет только Platform Owner.
- `loyalty_rules` разделяется на program, mechanics, reward definitions и immutable versions.
- `devices` развивается в smart links + tags + attribution events.
- Guest App разделяется на 4 экрана с общей mobile shell.
- Настройки гостевого приложения переводятся на draft → preview → publish → version history.
- Admin company creation заменяется транзакционным wizard command.

### Что удаляем из целевого UX

- Самостоятельное включение платных модулей владельцем бизнеса.
- Отдельные верхнеуровневые страницы API, файлы, аудит, интеграции, сайт и модули в Business-навигации.
- Случайные неработающие CTA и placeholder-разделы.
- PIN как основной Guest login. Он остаётся временным fallback до завершения OTP rollout.
- «Счастливое колесо» из MVP по умолчанию: только как entitlement-модуль после проверки экономики и законодательства.

### Критические найденные разрывы

1. Business Owner может вызывать `PATCH /modules/{code}`.
2. Staff имеет доступ к спискам модулей, филиалам, файлам, кампаниям и части аналитики шире целевой модели.
3. Нет единого subscription guard для изменяющих операций.
4. Нет серверных лимитов customers/staff/NFC/messages.
5. Нет master wizard, temporary-password flag и MFA Platform Owner.
6. Reward имеет слишком короткую модель и неполный lifecycle.
7. Нет support-session с TTL и обязательным audit banner.
8. Guest tokens хранятся в localStorage; production-цель — HttpOnly session cookie + CSRF для mutation.
9. Smart link не хранит source/campaign/location/UTM attribution как отдельные события.
10. WhatsApp sender существует, но retention event orchestration и approved-template governance не завершены.

## 1. Новая архитектура продукта

```text
Public Site (tappix.kz)
        │ lead / pricing
        ▼
Platform Control Plane (admin.tappix.kz)
  tenants ─ plans ─ subscriptions ─ entitlements ─ support sessions
        │ provisions
        ▼
Business Data Plane (app.tappix.kz)
  customers ─ visits ─ programs ─ rewards ─ messages ─ analytics
        │ publishes program + smart links
        ▼
Guest Experience (go.tappix.kz/{slug})
  OTP identity ─ progress ─ rewards ─ history ─ profile
```

Архитектурный стиль: modular monolith. Один deployable Go API, но строгие пакеты `platform`, `identity`, `tenant`, `billing`, `loyalty`, `messaging`, `smartlink`, `guest`, `audit`. Переход к микросервисам не нужен до появления измеримой нагрузки.

Каждый command проходит цепочку:

```text
Authenticate → Resolve tenant → Membership → Permission → Subscription
→ Entitlement → Limit → Validate input → Transaction → Audit → Outbox
```

События `CustomerRegistered`, `VisitRecorded`, `RewardAvailable`, `RewardRedeemed`, `LevelChanged` пишутся в transactional outbox. Worker доставляет WhatsApp и обновляет projections с idempotency key.

## 2. Карта ролей

- `platform_owner`: только Армат; control plane, MFA обязательно.
- `business_owner`: управляет разрешённой конфигурацией своего tenant.
- `business_manager` (опционально позже): аналитика и контент без пользователей/финансов.
- `business_staff`: кассовый операционный сценарий.
- `guest`: только собственная loyalty identity внутри конкретного tenant.
- `support_session`: не роль, а краткоживущий scoped grant Platform Owner для одного tenant.
- `system_worker`: внутренний principal для outbox/automation; не используется человеком.

## 3. Permission matrix

| Capability | Platform Owner | Business Owner | Staff | Guest |
|---|---:|---:|---:|---:|
| Создать/заблокировать tenant | ✓ | — | — | — |
| Тариф, модули, лимиты | ✓ | view | — | — |
| Создать owner/staff | ✓ | staff в лимите* | — | — |
| Произвольные роли | — | — | — | — |
| Настроить/опубликовать программу | support | ✓ | — | — |
| Просмотр клиентов tenant | support | ✓ | limited | — |
| Экспорт клиентов | support | ✓ + permission | — | — |
| Отметить визит | support | ✓ | ✓ | — |
| Ручная корректировка баланса | support | ✓ + reason | bounded | — |
| Выдать доступную награду | support | ✓ | ✓ | — |
| NFC/QR | ✓ limits | ✓ в лимите | scan only | open only |
| Массовые сообщения | ✓ governance | ✓ entitlement | — | — |
| Собственная карта/история | — | — | — | ✓ |

`*` Если ТЗ трактуется буквально — staff создаёт только Platform Owner. Рекомендуемая коммерческая модель: Business Owner приглашает staff только в пределах тарифа; роль и привилегии фиксированы.

Permission codes: `tenant.manage_profile`, `program.edit_draft`, `program.publish`, `customer.read`, `customer.export`, `visit.create`, `reward.redeem`, `balance.adjust_bounded`, `tag.manage`, `message.send`, `analytics.read`, `staff.invite_limited`.

## 4. Структура доменов

| Host | Назначение | Session audience |
|---|---|---|
| `tappix.kz` | маркетинг, цены, заявки | public |
| `app.tappix.kz` | Business Workspace и staff mode | business |
| `admin.tappix.kz` | Platform Control Plane | platform + MFA |
| `go.tappix.kz/{slug}` | smart link и Guest App | guest |
| `api.tappix.kz` | API/BFF endpoints | audience-scoped |
| `cdn.tappix.kz` | проверенные изображения и exports | signed/public policy |

Cookies не разделяются между admin/app/go. JWT получает обязательные `aud`, `session_id`, `membership_id`, `tenant_id`, `support_session_id?`.

## 5. Sitemap трёх интерфейсов

### Platform Owner

```text
/overview
/companies
  /new (5-step wizard)
  /{id}/overview
  /{id}/subscription
  /{id}/limits
  /{id}/users
  /{id}/support-session
/plans
/subscriptions
/payments
/users
/messaging/whatsapp
/support-sessions
/audit
/platform-settings
```

### Business Workspace

```text
/overview
/customers
/visits
/loyalty
  program | rewards | levels | offers | messages
/smart-links
/messages
/analytics
/settings
  company | team | guest-app | integrations | security | subscription(read-only)
```

`Reviews` появляется отдельно только при entitlement. Staff видит `/customers`, `/visits`, `/rewards/redeem`; остальные destinations дают 403 screen, а не пустой layout.

### Guest App

```text
/{slug}                 smart-link resolver
/{slug}/join            value proposition + OTP registration
/{slug}/home            return goal + compact card + latest activity
/{slug}/rewards         available | in progress | used
/{slug}/history         credits | debits | visits | rewards
/{slug}/profile         identity, consent, sessions, contacts
/{slug}/card            full-screen QR card
```

Bottom navigation: Главная, Награды, История, Профиль. NFC не является пунктом меню.

## 6. Целевая схема данных

```text
tenants 1─* memberships *─1 users
tenants 1─* locations
tenants 1─1 subscriptions *─1 plans
plans 1─* plan_entitlements
subscriptions 1─* subscription_overrides

tenants 1─* loyalty_programs 1─* loyalty_program_versions
program_versions 1─* reward_rules
reward_rules 1─* reward_definitions
customers 1─* guest_identities
customers 1─* visits
customers 1─* loyalty_transactions
customers 1─* customer_reward_progress
customers 1─* customer_rewards

tenants 1─* smart_links 1─* smart_link_events
smart_links *─0..1 locations
tenants 1─* message_templates
tenants 1─* message_logs
outbox_events → message worker
audit_logs, support_sessions, security_events
```

Ключевые таблицы:

- `plans`, `plan_entitlements(code, limit_value, enabled)`.
- `subscription_overrides` с actor, reason, valid_until.
- `loyalty_program_versions(status=draft|published|archived, config, published_at)`.
- `reward_definitions`: type, image, visits_required, points_required, validity, inventory, branches, segments, repeatable, cooldown, confirmation.
- `customer_reward_progress`: current/target, status.
- `customer_rewards`: locked/in_progress/available/reserved/redeemed/expired/cancelled.
- `loyalty_transactions`: immutable ledger; reversal создаёт новую запись, не редактирует старую.
- `smart_link_events`: source, tag, campaign, location, language, UTM, resolved state.
- `support_sessions`: tenant, actor, reason, starts/expires/revoked.

Все tenant-таблицы: `tenant_id NOT NULL`, composite indexes `(tenant_id, id/created_at)`, RLS и service-layer tenant filter.

## 7. Guest user flow

```text
Tap NFC / Scan QR / Referral URL
 → smart-link validation
 → tenant/subscription/tag resolution
 → existing HttpOnly guest session?
    ├─ yes + same tenant → Home
    └─ no → Branded value screen
          → Получить карту
          → phone + consent
          → OTP request/throttle
          → OTP verify
          → existing identity?
             ├─ yes → rotate session → Home
             └─ no → name + birthday (+ optional fields)
                    → create customer/identity
                    → initialize reward progress
                    → Welcome screen
                    → Home
```

Home priority:

1. «Остался 1 визит до подарка» + stamp progress + WhatsApp booking CTA.
2. Compact digital card; tap opens `/card` full-screen QR.
3. Реально доступные преимущества.
4. 1–2 relevant offers.
5. Последние 3 события.

Offline: cached shell + last known read-only card with «Данные могут быть неактуальны»; mutations disabled.

## 8. Reward lifecycle

```text
locked → in_progress → available → reserved → redeemed
                    ↘ expired
                    ↘ cancelled
```

- Progress processor реагирует на domain events и идемпотентно обновляет progress.
- Выполнение условия создаёт `available`, но не `redeemed`.
- Staff сканирует QR → сервер проверяет tenant, reward, status, branch, inventory, expiry, staff permission.
- Redeem выполняется в одной транзакции: lock row → decrement inventory → status redeemed → ledger effect → audit → outbox WhatsApp.
- Отмена создаёт compensating event; история не переписывается.
- `reserved` имеет TTL и освобождается worker-ом.

## 9. Макеты ключевых экранов

### Guest Home

```text
┌ Dentline                    bell  avatar ┐
│ Здравствуйте, Мадина                     │
├──────────────────────────────────────────┤
│ ДО ПОДАРКА ОСТАЛСЯ 1 ВИЗИТ               │
│ ●  ●  ●  ●  ○                            │
│ Профессиональная чистка в подарок        │
│ [ Записаться через WhatsApp ]            │
├──────────────────────────────────────────┤
│ compact card: имя · Bronze · 120 · QR    │
│ [ Показать карту ]                       │
├──────────────────────────────────────────┤
│ Доступно сейчас                          │
│ 120 бонусов = до 1 200 ₸                 │
│ Бесплатная консультация                  │
├──────────────────────────────────────────┤
│ Для вас: одно релевантное предложение    │
├──────────────────────────────────────────┤
│ Последняя активность (3 события)         │
└ Home ─ Rewards ─ History ─ Profile ──────┘
```

### Business Overview

```text
Сегодня: returning customers · visits · rewards redeemed
Action center: NFC не настроен / программа draft / 3 rewards ждут выдачи
Retention trend: first → repeat visit conversion
Recent visits + быстрые действия
```

### Guest App Editor

```text
┌ Settings (draft) ─────────┬ Phone preview ┐
│ Brand                     │ live safe UI  │
│ Return mechanic           │               │
│ Next reward               │               │
│ Contacts / WhatsApp       │               │
│ Block order               │               │
│ [Preview] [Publish]       │               │
└───────────────────────────┴───────────────┘
```

### Master Company Wizard

```text
Company → Owner → Plan & limits → Modules → Review
          autosaved draft          transactional create
```

## 10. План миграции без потери данных

### Phase A — safety baseline

1. Production backup + restore rehearsal.
2. Добавить migration ledger (`schema_migrations`); прекратить повторный запуск всех SQL файлов.
3. Contract tests текущих routes и tenant isolation.

### Phase B — additive schema

1. Создать новые plans/entitlements/program versions/reward progress/smart links/outbox/support sessions.
2. Добавлять nullable/backfilled columns; старые поля не удалять.
3. Backfill `tenant_id=company_id`, plans из subscriptions, reward definitions из loyalty rules.

### Phase C — dual read/write

1. Новые commands пишут новый ledger/outbox и совместимые старые projections.
2. Shadow-read сравнивает старые и новые результаты.
3. Feature flags включаются tenant-by-tenant, начиная с тестового tenant.

### Phase D — authorization cutover

1. Централизованный policy engine.
2. Server entitlement/limit middleware.
3. Запрет Business Owner module mutation.
4. Staff least privilege tests и 403 UI states.

### Phase E — interface split

1. Отдельные root layouts и audiences для admin/app/go.
2. Новый Master wizard.
3. Business IA и staff mode.
4. Guest 4-tab shell и smart links.

### Phase F — retention automation

1. Transactional outbox worker.
2. Approved WhatsApp templates, quotas, idempotency and logs.
3. Visit/reward/birthday/inactivity/expiry events.

### Phase G — deprecation

1. 30 дней telemetry старых routes.
2. Отключить старые writes.
3. Экспорт/архив, затем отдельная destructive migration только после backup approval.

## API contracts v3 (контур)

- Platform: `/v3/platform/tenants`, `/plans`, `/subscriptions`, `/support-sessions`.
- Business: `/v3/workspace/*`; tenant берётся только из membership/session.
- Guest public: `/v3/go/{slug}/resolve`, `/otp/request`, `/otp/verify`.
- Guest private: `/v3/guest/home`, `/rewards`, `/history`, `/profile`, `/card-token`.
- Staff: `/v3/staff/customers/lookup`, `/visits`, `/rewards/{id}/redeem`.
- Mutations принимают `Idempotency-Key`; ошибки имеют `code`, `message`, `requestId`, `details?`.

## Subscription model

Guard возвращает snapshot:

```json
{
  "status": "active",
  "entitlements": {"rewards": true, "whatsapp": false},
  "limits": {"customers": 1000, "staff": 5, "smartLinks": 10, "messagesMonthly": 500},
  "usage": {"customers": 842, "staff": 4, "smartLinks": 8, "messagesMonthly": 421}
}
```

Expired subscription: read-only grace period по policy; все writes получают `SUBSCRIPTION_EXPIRED`. Limit breach: `LIMIT_REACHED` с upgrade contact, но без возможности self-upgrade.

## Threat model

| Threat | Control |
|---|---|
| IDOR/cross-tenant | session tenant + membership + query predicate + RLS + tests |
| Role escalation | fixed roles, no client role assignment, policy engine |
| Module/limit bypass | server entitlement + transactional usage check |
| OTP brute force | hash, 5 attempts, phone/IP/device throttle, short TTL |
| Token theft | HttpOnly Secure SameSite cookies, rotation, session revoke |
| CSRF | SameSite + CSRF token for cookie-auth mutations |
| Support abuse | MFA, explicit reason, 30-min grant, banner, audit |
| Reward double spend | row lock, idempotency key, immutable ledger |
| Webhook spoofing | signature + timestamp + replay protection |
| Malicious upload | MIME sniff, size limit, randomized key, image re-encode |
| Secret leak | secrets manager, rotation, redacted logs |

## Design system direction

- B2B: `#F5F7FA`, white surfaces, `#111827` navigation, `#2563EB` action, `#14B8A6` secondary.
- Guest: brand-controlled accent inside constrained semantic tokens; dark and light themes both AA.
- 8px grid; radii 10/14/20; controls 44/48px; focus ring 3px; motion 150–300ms.
- No arbitrary CSS. Published themes validate contrast, image dimensions and token ranges.
- Required states: loading/skeleton, empty, error/retry, success, forbidden, expired, limit reached, offline.

## Решения, требующие утверждения

1. Может ли Business Owner приглашать staff в пределах лимита? Рекомендация: да.
2. Grace period после окончания подписки: рекомендация 7 дней read-only.
3. Основная MVP-механика: рекомендация stamp/visits + конкретный подарок; баллы вторичны.
4. «Колесо удачи»: рекомендация убрать из core MVP и оставить entitlement-модулем.
5. Guest session: рекомендация HttpOnly cookie на `go.tappix.kz`, 90 дней, device revoke.

