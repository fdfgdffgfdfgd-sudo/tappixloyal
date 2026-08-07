# Tappix: принятые архитектурные решения

## Границы MVP

Первый рабочий поток: владелец компании входит в систему → находит клиента → добавляет посещение → в одной транзакции создаются посещение и запись бонусного ledger → обновляется баланс → действие попадает в аудит.

## Multi-tenancy

`company_id` обязателен во всех бизнес-таблицах. Изоляция выполняется в трёх слоях:

1. tenant берётся только из проверенного access token, а не из query/body;
2. repository всегда принимает `company_id` из request context;
3. PostgreSQL RLS использует транзакционную переменную `app.company_id` как дополнительную защиту.

Super Admin работает через отдельные системные use cases и не отключает tenant-фильтры в обычных company endpoints.

## Loyalty

`bonus_ledger` неизменяемый. `customers.total_points` — быстрая read-модель, обновляемая в той же транзакции. Повторная отправка защищена `idempotency_key`. Правила хранят условия и действия в JSONB, поэтому welcome bonus, visit bonus и birthday bonus не требуют изменения схемы.

## Следующие реализации

1. Подключение `pgx` и транзакционного tenant scope.
2. JWT access token + refresh rotation в Redis.
3. Customer repository/service/handler.
4. Visit service с блокировкой клиента `FOR UPDATE`.
5. OpenAPI и интеграционные тесты на утечку данных между tenant A/B.
