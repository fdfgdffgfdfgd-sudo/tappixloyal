# Tappix Loyalty Platform

Multi-tenant SaaS-платформа лояльности: CRM, посещения и бонусы, клиентский кабинет, кампании, аналитика, управление филиалами и сотрудниками, сайты и онлайн-запись, подписки и Super Admin.

## Запуск

```bash
npm ci
docker compose up -d --build
npm run db:migrate
```

Бизнес-панель: `http://localhost:8088`. API: `http://localhost:8080/api/v1`. Тестовая почта: `http://localhost:8025`.

Перед внешним развёртыванием скопируйте `.env.example` в `.env`, задайте уникальные `POSTGRES_PASSWORD`, `JWT_SECRET`, SMTP и публичный `APP_URL`. Файл `.env` не попадает в репозиторий.

Для разработки только frontend используйте `npm run dev` и откройте `http://localhost:3000`.

## Проверки

```bash
npm run test:api
npm run lint
npm run build
npm run test:integration
```

Команда `npm test` последовательно запускает все проверки; для интеграционного этапа Docker-стек должен быть запущен и миграции применены. Workflow `.github/workflows/ci.yml` выполняет эти проверки автоматически для push и pull request.

Резервная копия PostgreSQL создаётся командой `npm run db:backup` в каталоге `backups/`. Восстановление требует явного подтверждения: `CONFIRM_RESTORE=tappix sh infrastructure/scripts/restore-postgres.sh /exact/path/backup.sql.gz`.

## Структура

- `apps/business` — Next.js бизнес-панель владельца и сотрудников.
- `apps/api` — Go API и SQL-миграции PostgreSQL.
- `docs` — OpenAPI и проектная документация.
- `infrastructure` — Nginx и интеграционные тесты.
- `docker-compose.yml` — полный локальный стек: web, API, PostgreSQL, Redis и Mailpit.
