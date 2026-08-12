# Tappix Design System

Канонический источник правил интерфейса Tappix. Page-specific overrides допустимы только в `design-system/pages/` и не могут ослаблять accessibility, security или responsive requirements.

## Product principles

- Каждый экран решает одну основную задачу пользователя.
- Business UI использует понятный язык, без backend-терминов.
- Не смешивать активные механики лояльности в одном hero.
- Максимум два визуальных уровня контейнеров.
- Empty state объясняет, что появится и что сделать дальше.
- Никакого glassmorphism, декоративного шума и нефункциональных градиентов.

## Semantic tokens

```css
:root {
  --app-bg: #f8fafc;
  --surface: #ffffff;
  --surface-subtle: #f1f5f9;
  --text: #0f172a;
  --text-muted: #64748b;
  --border: #e2e8f0;
  --accent: #6352ee;
  --accent-hover: #5140da;
  --accent-soft: #eef0ff;
  --success: #16794b;
  --success-soft: #ecfdf3;
  --warning: #a15c07;
  --warning-soft: #fff7e6;
  --danger: #b42335;
  --danger-soft: #fff1f2;
  --focus: #6352ee;
}
```

Branding клиента разрешено только через валидируемый accent/logo/cover. Arbitrary CSS запрещён. Контраст текста проверяется автоматически; при небезопасном brand color используется системный fallback.

## Typography

- Self-hosted Inter либо системный sans-serif fallback.
- Heading: 600–750; body: 400–550; controls: 600–750.
- Page title: `clamp(24px, 2vw, 32px)`.
- Section title: 18–22px; body: 14–16px; helper: не меньше 12px.
- Не использовать runtime Google Fonts.

## Spacing and layout

- База: 4px; рабочий rhythm: 8px.
- Scale: 4, 8, 12, 16, 20, 24, 32, 40, 48, 64.
- Sidebar: 252px, sticky; main max-width: 1520px.
- Desktop grid: 12 columns; gutters 24–32px.
- Breakpoints: 1280/1440/1920 desktop, 768–1024 tablet; guest 320/360/375/390/430.
- На 375px нет горизонтального scroll; при 200% zoom задача остаётся выполнимой.

## Shape and elevation

- Radius: controls 9–11px, sections 14–18px, full-screen customer hero до 24px.
- Border: 1px `--border`.
- Shadows только для overlay: `0 24px 70px rgba(15,23,42,.18)`.
- Cards не двигаются и не масштабируются при hover.

## Components

Единые primitives: `PageHeader`, `SectionHeader`, `MetricCard`, `StatusBadge`, `EmptyState`, `InfoCallout`, `StepIndicator`, `MechanicSelector`, `RewardSummary`, `CustomerProgress`, `CustomerLevel`, `CustomerQRModal`, `CustomerCode`, `ReferralFlow`, `ReferralSummary`, `SettingsSection`, `StickyActionBar`, `ConfirmDialog`, `Skeleton`, `ErrorState`, `LockedFeature`.

Controls:

- minimum target 44×44px;
- один primary CTA на экран/диалог;
- disabled блокирует действие и визуально объясним;
- focus ring: `3px solid color-mix(in srgb, var(--focus), transparent 70%)`;
- validation показывается под полем, не только toast-ом;
- submit блокируется на время запроса;
- опасные и финансовые действия требуют confirmation.

## Admin shell

Группы: Работа, Лояльность, Коммуникации, Аналитика, Система. Активен только реальный текущий пункт. Staff видит только операционные destinations. Недоступные функции скрываются либо получают один аккуратный locked state при наличии upsell-смысла.

## Loyalty patterns

- Основные сценарии: «Подарок за посещения», «Бонусы с покупок», «Скидка растёт с клиентом».
- Program editor: способ прогресса → награда → дополнительные правила → preview → publish summary.
- Preview открывается по запросу, а не занимает постоянную треть страницы.
- Referral setup сначала визуально объясняет путь, затем показывает базовые награды; antifraud находится в advanced section.

## Guest shell

- Hero показывает только активную механику, прогресс и следующую награду.
- QR не находится постоянно на главной; CTA «Показать карту на кассе» открывает full-screen QR.
- Под QR всегда стабильный 6-значный customer code и copy action.
- Bottom nav: Главная, Награды, История. Safe-area обязателен.
- История использует человеческие названия операций.

## States

- Loading: skeleton повторяет будущий layout.
- Empty: причина + содержимое в будущем + одно следующее действие.
- Error: конкретное сообщение + Retry.
- Success: краткий `aria-live` toast/status.
- Paid module: `available`, `configured`, `connected`, `locked`, `failed`.
- Expired subscription: read-only state, backend остаётся source of truth.

## Accessibility and motion

- WCAG AA: 4.5:1 normal text, 3:1 large text/UI graphics.
- Полный keyboard flow, `:focus-visible`, logical focus order и focus return.
- Modal: `role=dialog`, `aria-modal`, labelled title, focus trap, Escape и click-outside policy.
- Errors/success используют `aria-live`; labels не заменяются placeholder-ами.
- `prefers-reduced-motion` отключает non-essential motion.
- Animation 150–200ms; loading spinner может быть дольше.
- SVG/Lucide вместо emoji как структурных иконок.

## Performance

- Server pagination/search; не загружать списки клиентов целиком.
- Lazy-load camera, heavy charts и secondary flows.
- AVIF/WebP и заданные размеры изображений.
- Self-hosted fonts; bundle и Core Web Vitals контролируются в production.
- Не дублировать критичные расчёты eligibility/progress в frontend.

## Definition of done

UI и API работают; tenant isolation, entitlement и RBAC проверены; loading/empty/error/success реализованы; unit, integration и e2e happy path проходят; migration проверена; responsive/keyboard/WCAG flow проверены; lint, typecheck, build и Docker проходят; telemetry и audit присутствуют у критичных операций.
