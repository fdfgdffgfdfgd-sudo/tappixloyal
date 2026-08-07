# Tappix Product Architecture v2

## 1. Product thesis

Tappix is not a generic CRM. It is a multi-tenant NFC loyalty operating system sold by subscription. Its primary daily loop is:

`NFC touch → customer identity → visit → reward → review → measurable return`

The product has two strictly separated applications:

- **Tappix Platform** — the founder and platform team operate tenants, revenue, plans, support, risk and infrastructure.
- **Tappix Business** — a company owner and team operate one selected workspace and never see platform controls.

Customer-facing registration, loyalty balance and review flows form a third lightweight surface, **Tappix Customer**, with no business navigation.

## 2. Product principles

1. Every business screen must advance activation, retention, customer service or revenue visibility.
2. A route represents a durable place; a modal represents a short action.
3. One primary action per screen. Common actions remain reachable in one or two clicks.
4. Progressive disclosure: advanced configuration lives inside Settings or an entity detail, not the main navigation.
5. Tenant context is explicit, visible and enforced server-side on every request.
6. Empty states teach the next step and are part of onboarding.
7. Platform and Business share design tokens, not navigation or permissions.

## 3. Tenancy and domain architecture

### Identity model

- `users` is a global identity: one email/password, many workspaces.
- `companies` is the tenant/workspace.
- `company_memberships` links a user to a company with `owner`, `admin`, `manager` or `staff` role.
- Access tokens carry `userId`, `activeCompanyId`, `membershipRole` and `platformRole`.
- Switching a workspace issues a new tenant-scoped access/refresh pair after membership validation.
- All tenant tables keep `company_id`; PostgreSQL RLS and repository predicates provide defense in depth.
- `super_admin` is a platform role and does not become a tenant member implicitly. Impersonation creates a short-lived, auditable support session with a visible banner.

### Product domains

- **Identity & Access:** login, recovery, sessions, memberships, invitations, impersonation.
- **Tenant Management:** companies, workspace profile, branches, employees, limits.
- **Customer CRM:** people, segments, history, import/export.
- **Loyalty:** points, visits, tiers, rewards, campaigns and automation.
- **NFC & QR:** physical devices, destinations, activation, conversion and orders.
- **Reviews:** collection funnel, routing, provider links and reputation analytics.
- **Analytics:** activation, visits, redemption, retention, NFC conversion and staff activity.
- **Billing:** plans, subscriptions, invoices, payments, limits and entitlements.
- **Platform Operations:** support, audit, logs, API keys, health and global settings.

## 4. Roles and permissions

### Platform roles

- **Founder / Super Admin:** full platform access, company lifecycle, billing, plans, limits, support impersonation and logs.
- **Platform Support:** read tenants and users, open support sessions; cannot alter plans or delete companies.
- **Platform Finance:** plans, invoices, payments and revenue; cannot impersonate or edit tenant data.

### Business roles

- **Owner:** company settings, billing, memberships, all loyalty and analytics capabilities.
- **Admin:** operational configuration and team management; no ownership transfer or billing method.
- **Manager:** customers, loyalty, NFC, reviews and analytics; no company/security settings.
- **Staff:** customer lookup, visit check-in, point operations allowed by policy and review handoff.

### Customer role

- **Customer:** only their profile, balance, rewards, visits and review journey for the current brand.

## 5. Information architecture

### Tappix Platform navigation

1. **Overview** — companies, MRR, payment risk, activation and platform health.
2. **Companies** — tenant lifecycle, detail, limits, plan and audited support login.
3. **Users** — global identities, memberships, status and sessions.
4. **Revenue** — tabs: Subscriptions, Payments, Orders, Plans.
5. **Support** — tickets, incidents and impersonation history.
6. **Insights** — acquisition, churn, plan distribution and cohort metrics.
7. **Developer** — tabs: API, webhooks, logs and system health.
8. **Platform settings** — brand, email, security and global defaults.

### Tappix Business navigation

Exactly seven primary destinations:

1. **Overview**
2. **Customers**
3. **Loyalty**
4. **NFC**
5. **Reviews**
6. **Analytics**
7. **Settings**

Secondary features are nested:

- Loyalty: rules, rewards, campaigns, automation.
- NFC: devices, QR, destinations, orders.
- Settings: company, branches, team, billing, integrations, API, files, website, booking, security and audit.

### Workspace switcher

The top-left workspace control shows logo, current company and plan. Its menu contains:

- searchable list of available companies;
- create company;
- company settings;
- invite team;
- switch to Platform only when the identity has a platform role;
- logout, visually separated from workspace actions.

## 6. Screen map and relationships

```text
/platform
├── overview
├── companies ── company detail ── plan | limits | users | support login
├── users ── user detail ── memberships | sessions
├── revenue ── subscriptions | payments | orders | plans
├── support ── ticket detail
├── insights
├── developer ── api | webhooks | logs | health
└── settings

/app/[workspace]
├── overview
│   ├── quick-action modals
│   └── activity → customer / visit / review detail
├── customers ── customer detail
├── loyalty ── rules | rewards | campaigns | automation
├── nfc ── device detail | destinations | orders
├── reviews ── inbox | settings
├── analytics ── acquisition | retention | loyalty | NFC
└── settings ── company | branches | team | billing | integrations | developer | security

/c/[workspace]
├── join/[device-token]
├── account
├── rewards
└── review/[visit-token]
```

Short actions use accessible dialogs: create customer, add visit, adjust points, create NFC, create campaign, invite employee, create company and change plan. Entity editing may use a side sheet when context must remain visible. Destructive actions always require a confirmation dialog.

## 7. Core user journeys

### Journey A — first activation for a dental clinic

Purchase → invitation email → create password → onboarding checklist → confirm company → invite receptionist → activate NFC device → test customer registration → first real registration → first visit → automatic points → review request → Overview shows first conversion.

### Journey B — receptionist daily check-in

Open Overview → global customer search → select customer → “Add visit” modal → choose branch/service → preview earned points → confirm → receipt state offers review handoff. Target: under 20 seconds.

### Journey C — owner morning review

Open Overview → scan today metrics → inspect anomalies → latest registrations/visits → staff activity → open Analytics only for deeper comparison. No setup pages interrupt this loop.

### Journey D — multi-brand owner

Open workspace switcher → type brand name → switch tenant → access token is re-scoped → current route resolves to same feature where possible → all metrics and actions reflect the new company.

### Journey E — loyalty campaign

Loyalty → Campaigns tab → Create campaign modal → choose segment → preview audience → compose and test → schedule/send → progress and delivery report → segment performance in Analytics.

### Journey F — NFC rollout

NFC → Create device modal → select branch and destination → assign label → copy QR/encoding link → mark installed → live conversion funnel records touches, registrations and visits → low-performing device flagged.

### Journey G — reputation recovery

Reviews inbox → low rating highlighted → open customer context → assign follow-up → record resolution → high ratings are routed to configured public provider → owner sees recovery rate.

### Journey H — team administration

Settings → Team → Invite modal → role preset explains permissions → employee accepts invitation → owner can change role, restrict branch or revoke access → action appears in audit history.

### Journey I — founder provisions a tenant

Platform Companies → Create company modal → owner identity/invitation → plan and limits → optional NFC order → workspace created atomically → founder sees activation checklist and can open an audited support session.

### Journey J — payment risk

Platform Overview alerts past-due tenant → company detail → inspect invoices and attempts → contact or grant grace period → change status → business owner sees a non-blocking billing banner before suspension.

### Journey K — customer experience

Tap NFC → branded join page → phone/PIN registration → welcome reward → lightweight account shows balance and next reward → after visit customer receives review link → no administrative UI is exposed.

### Journey L — support investigation

Support ticket → tenant context → logs and recent audit → start time-limited support session with reason → visible impersonation banner → reproduce issue → exit session → immutable audit entry.

## 8. Business Overview specification

The page answers three questions: “What happened today?”, “Does anything need attention?”, “What should I do next?”

- KPI row: registrations today, visits today, points issued, points redeemed, NFC conversion.
- Attention strip: incomplete onboarding, disconnected NFC, payment risk or failed campaign.
- Quick actions: customer, visit, points and NFC.
- Activity split: latest registrations and visits.
- Reputation card: review score, new reviews and unresolved low ratings.
- Team activity: top operators and recent sensitive actions.
- Trend chart: visits and registrations with a 7/30-day selector and accessible table alternative.

New tenants see an activation checklist instead of empty charts. Mature tenants see operational data first.

## 9. Visual and interaction system

- Tone: precise, quiet, premium and operational; no decorative dashboard clutter.
- Surfaces: warm white canvas, white cards, subtle cool-gray borders, restrained navy text and one blue accent.
- Typography: Inter for product UI; tabular numerals for metrics. Display typography is reserved for marketing, not dense admin screens.
- Radius: 10px controls, 14px cards, 16px dialogs; shadows only for overlays.
- Density: compact desktop with 44px minimum targets; comfortable mobile layout.
- Motion: 150–220ms opacity/translate transitions, disabled under reduced-motion.
- Feedback: skeleton after 300ms, inline form errors, `aria-live` status, optimistic UI only for reversible actions.
- Responsive: desktop sidebar, tablet compact rail, mobile top bar plus up to five primary destinations; secondary items live in “More”.

## 10. Delivery sequence

1. Introduce memberships and tenant switching without breaking existing tenant isolation.
2. Build new Business shell with seven destinations and workspace switcher.
3. Replace Overview with action-oriented dashboard and onboarding state.
4. Consolidate existing routes into tabs under Loyalty, NFC and Settings.
5. Build independent Platform shell and expand company lifecycle operations.
6. Convert short create flows to shared accessible dialogs.
7. Add audited impersonation and advanced limits.
8. Run role, tenant-isolation, keyboard, mobile and integration regression suites.

## 11. Success metrics

- Time to first NFC activation.
- Time from login to first visit entry.
- Companies completing activation in 24 hours.
- Weekly active owners and staff.
- NFC touch-to-registration conversion.
- Registration-to-second-visit retention.
- Reward redemption and review completion.
- Trial-to-paid conversion, MRR, churn and past-due recovery.
