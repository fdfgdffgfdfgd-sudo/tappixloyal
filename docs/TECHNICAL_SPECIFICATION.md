# Техническое задание Tappix

Версия: 1.0  
Назначение: единый документ для совместной разработки продукта, API, интерфейсов и интеграций.

## 1. Цель продукта

Tappix — мультиарендная B2B SaaS-платформа лояльности для малого и среднего бизнеса. Платформа должна позволять компании самостоятельно за 10–15 минут запустить программу, принимать операции через NFC/QR/POS, управлять клиентами и коммуникациями и видеть доказуемый финансовый результат.

Ключевой продуктовый результат: владелец бизнеса понимает не только число регистраций и бонусов, но и влияние программы на повторные покупки, средний чек, удержание, LTV и выручку.

## 2. Роли и границы доступа

- Platform Owner: компании, тарифы, системные настройки, аудит, support sessions.
- Business Owner: все данные своей компании, подписка, сотрудники, интеграции и аналитика.
- Manager: операционная работа в разрешённых разделах без управления тарифом и критичными доступами.
- Staff: упрощённый Staff Mode — поиск клиента, чек, бонусы и последние операции.
- Customer: клиентская карта, баланс, награды, история, согласия и профиль.
- Guest: знакомство с продуктом и демоданными без доступа к реальным данным.

Все серверные запросы обязаны проверять `company_id`. Филиал, клиент, сотрудник, кампания и интеграционное подключение должны принадлежать одному tenant.

## 3. Тарифы и активация возможностей

### Starter

- CRM и карточка клиента.
- Базовая программа лояльности.
- NFC/QR-регистрация.
- Отзывы.
- Базовые операционные показатели.

### Growth

Всё из Starter, дополнительно:

- расширенная аналитика, RFM и retention;
- кампании Email/SMS/Telegram;
- сайт и онлайн-запись;
- партнёрства и рефералы;
- POS Integration Hub;
- регулярные отчёты;
- автоматические триггеры.

### Pro

Всё из Growth, дополнительно:

- публичный API и API-ключи;
- webhooks;
- расширенные лимиты;
- ROI, holdout и uplift;
- приоритетная поддержка и расширенный аудит.

### Обязательное поведение переключения тарифа

1. Тариф меняется одной атомарной серверной операцией.
2. Сервер нормализует тариф и сам включает стандартный набор модулей. Клиент не является источником истины для entitlements.
3. `/subscription` возвращает тариф, модули, entitlements и лимиты.
4. `/modules` возвращает для каждой возможности `enabled`, `available`, `requiredPlan`.
5. После повышения тарифа новые разделы доступны без повторного входа; интерфейс обновляет session/subscription cache.
6. После понижения данные не удаляются. Недоступные функции переходят в read-only или показывают upgrade state.
7. «Доступно по тарифу» и «подключено» — разные состояния. Например, Growth открывает Poster, но пользователь ещё должен пройти OAuth или указать API token.
8. Ошибка интеграции должна объяснять следующий шаг: «Функция доступна. Подключите Poster», а не «Не сделано».
9. Все ограничения проверяются сервером, а не только скрытием кнопок.
10. Каждая смена тарифа записывается в audit log.

Критерий приёмки: после Starter → Growth API и UI одновременно показывают включённые Growth-модули; после Growth → Pro появляется API-доступ; после downgrade защищённый endpoint возвращает структурированную ошибку `PLAN_UPGRADE_REQUIRED`.

## 4. Guided Onboarding

Мастер запуска:

1. отрасль;
2. бизнес-цель;
3. рекомендуемый шаблон;
4. экономика бонусов;
5. логотип и цвета;
6. филиал;
7. сотрудник;
8. NFC/QR;
9. тестовый клиент;
10. тестовый чек;
11. публикация.

Тестовые сущности должны иметь `is_test=true` и не попадать в бизнес-аналитику. Прогресс сохраняется на сервере. Пользователь может продолжить с последнего завершённого шага.

Шаблоны: кофейня, салон, ресторан, автосервис, клиника, retail.

## 5. Клиенты и лояльность

- единая карточка клиента и серверный поиск по телефону/имени;
- нормализация телефона E.164 и предотвращение дублей;
- уровни, cashback, штампы, фиксированные награды;
- начисление, списание, возврат и ручная корректировка с причиной;
- баланс, срок сгорания и денежный эквивалент;
- журнал операций и аудит сотрудника;
- согласия и предпочтительные каналы связи;
- объединение дублей без потери истории.

## 6. Канонический API чеков

Endpoints:

```text
POST /integrations/transactions/quote
POST /integrations/transactions
POST /integrations/transactions/{id}/refund
GET  /integrations/transactions/{id}
POST /integration-jobs/{id}/retry
```

Требования:

- API keys хранятся только в виде hash и имеют отдельные scopes;
- поддерживаются sandbox и production;
- `external_id` обязателен;
- уникальность `UNIQUE(company_id, provider, external_id)`;
- повтор одного запроса идемпотентен;
- поддерживаются полный и частичный возврат;
- quote ничего не записывает в финансовый ledger;
- операция сохраняет raw payload для диагностики с маскированием секретов;
- принадлежность всех связанных сущностей tenant проверяется до записи.

## 7. POS Integration Hub

Первый провайдер — Poster:

- OAuth/API token;
- импорт филиалов и их сопоставление;
- импорт клиентов;
- backfill чеков за 90 дней;
- webhook новых и возвращённых чеков;
- ежедневная reconciliation;
- retry и dead-letter queue;
- экран здоровья подключения и история синхронизаций.

Следующие адаптеры: iiko/Syrve, МойСклад, r_keeper, 1С. Kaspi реализуется как отдельный платёжный источник.

Все адаптеры реализуют единый `POSAdapter`: validate credentials, list locations, pull customers, pull transactions, normalize transaction, reconcile.

## 8. Аналитика

Вкладки:

- Результаты;
- Удержание;
- RFM;
- Кампании;
- Филиалы;
- Бонусные обязательства.

Обязательные метрики: repeat purchase rate, средний чек участников/неучастников, active customers, historical LTV, churn risk, RFM, выручка по филиалам, воронка NFC/QR, ROI кампаний, attributed revenue, liability.

Фоновый worker обновляет `analytics_daily_facts` и `analytics_customer_features` после чека, после возврата, ночью и по защищённому административному endpoint. Тяжёлые метрики не должны вычисляться при каждом HTTP-запросе.

Retention cohorts:

- регистрация: M0–M12;
- первая покупка: W0–W12;
- фильтры: филиал, источник, программа, шаблон, участие в лояльности, acquisition campaign.

Аналитика по тарифам:

- Starter: базовые цифры и короткие рекомендации;
- Growth: retention, RFM, филиалы, churn и автоматические insights;
- Pro: ROI, holdout/uplift, прогнозы, выгрузки и API.

## 9. Bonus FIFO и liability

Каждое начисление создаёт `bonus_lot` с суммой, остатком, денежной стоимостью, датой активации и сгорания. При списании расходуются старейшие активные лоты. Связь списания с лотами хранится отдельно.

Возврат восстанавливает исходные лоты. Worker сжигает просроченные остатки. Метрики: issued, activated, redeemed, expired, remaining liability, expected redemption cost.

## 10. Кампании и триггеры

- сегмент аудитории;
- канал и шаблон;
- стоимость сообщения и награды;
- attribution window;
- holdout 5–10%;
- delivery/open/click/conversion;
- attributed и incremental revenue;
- ROI и redemption rate.

Триггеры: день рождения, сгорание бонусов через три дня, «Мы скучаем» после 30 дней без визита. Каждая отправка идемпотентна и проходит через outbox.

Если holdout отсутствует, UI показывает только «атрибутированная выручка» и не называет её uplift или дополнительной выручкой.

## 11. Рефералы и партнёрства

Рефералы: награды обеим сторонам, квалификация после оплаченной покупки, минимальный чек, задержка выплаты, лимиты, ссылка/QR, WhatsApp-тексты, рейтинг амбассадоров, отмена после возврата, anti-fraud и ручная проверка.

Партнёрства: две компании создают согласованную программу; применение промокода у партнёра A создаёт проверяемое вознаграждение у партнёра B. Нужны лимиты, срок действия, взаимное подтверждение, журнал взаиморасчётов и защита от self-referral.

## 12. Staff Mode и клиентская карта

Staff Mode: сканирование QR, поиск клиента, баланс, quote списания, фиксация покупки, выдача награды, последние операции. Нет доступа к аналитике, тарифам, интеграциям, файлам и массовым рассылкам.

Клиентская карта: QR клиента, следующая награда, денежная стоимость и срок сгорания бонусов, акции, фильтры истории, согласия, каналы, удаление профиля, поддержка RU/KK/EN. Wallet passes — отдельный последующий этап.

## 13. Регулярные отчёты

Частота: ежедневно, еженедельно, ежемесячно. Каналы: email и WhatsApp. Форматы: CSV, XLSX, PDF. Состав: филиалы, repeat rate, выручка участников, liability, churn, кампании. Отправка должна иметь retry, idempotency и audit trail.

## 14. Минимальная модель данных

Основные таблицы:

```text
sales_transactions, sales_transaction_items, payments
customer_events, customer_attributions
integration_connections, integration_location_mappings
integration_customer_links, integration_sync_cursors
integration_jobs, integration_failures
campaign_conversions
referral_programs, referral_attributions, referral_rewards
bonus_lots, bonus_lot_redemptions
analytics_daily_facts, analytics_customer_features
report_schedules
company_modules, plan_entitlements, audit_logs
```

Денежные значения хранятся в decimal/minor units, но не float. Время — `timestamptz` в UTC. JSONB используется для provider payload, а не вместо нормализованных ключевых полей.

## 15. Безопасность и надёжность

- единый subscription/entitlement guard;
- RBAC и ограничение staff;
- MFA для Platform Owner;
- support sessions с TTL и аудитом;
- HttpOnly/Secure/SameSite cookies;
- CSRF-защита;
- HMAC-подпись webhook и защита от replay;
- rate limiting;
- outbox, idempotency и dead-letter;
- секреты не попадают в логи;
- резервные копии и проверка восстановления;
- structured logs, metrics, tracing и error monitoring.

## 16. UX, доступность и производительность

- единая навигация и корректный active state;
- skeleton, retry error, actionable empty state, success toast;
- состояние locked/available/configured/connected/failed;
- WCAG AA, `focus-visible`, keyboard navigation, focus trap;
- touch target от 44×44 px;
- отсутствие горизонтального скролла на 375 px;
- масштаб текста 200%;
- `prefers-reduced-motion`;
- self-hosted fonts, AVIF/WebP, lazy loading;
- server pagination/search;
- Core Web Vitals и error monitoring.

## 17. Тестирование и Definition of Done

Для каждого изменения обязательны:

1. unit-тест бизнес-логики;
2. integration-тест API и tenant isolation;
3. migration up на копии схемы;
4. e2e happy path и критические ошибки;
5. проверка responsive и keyboard flow;
6. отсутствие ошибок lint/typecheck/build;
7. документация endpoint и переменных окружения;
8. audit и telemetry для критичных операций.

Функция считается готовой, когда она работает через UI и API, защищена правами и тарифом, имеет пустое/loading/error/success состояния, покрыта тестом и пригодна для диагностики в production.

## 18. Рекомендуемый порядок работ

1. довести subscription activation и серверные guards;
2. закрыть канонический API чеков и возвратов;
3. analytics projection worker;
4. FIFO liability;
5. retention cohorts;
6. Poster import, webhook и reconciliation;
7. ROI/holdout;
8. рефералы и партнёрства;
9. отчёты;
10. Guided Onboarding;
11. Staff Mode и клиентская карта;
12. production hardening и масштабирование.
