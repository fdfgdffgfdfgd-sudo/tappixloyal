# Tappix MVP Information Architecture

## Product boundary

Tappix MVP performs five jobs: register a customer through NFC/QR, record a visit, issue/redeem loyalty value, collect a review and show whether the program works. A capability that does not improve one of these jobs is not primary navigation.

## Primary navigation

1. Overview — today, activation and quick actions.
2. Customers — search, profile, visits and balance.
3. Loyalty — rules, rewards and point operations.
4. NFC & QR — touchpoints and conversion.
5. Reviews — reputation funnel and provider routing.
6. Analytics — acquisition, visits, retention and loyalty outcomes.
7. Settings — company structure, access, billing and extensions.

## Progressive disclosure

- New workspaces see only the seven primary destinations.
- Branches and Team live under Settings and are reached from onboarding when needed.
- Email, Telegram, WhatsApp, API, Booking, Mini Site, Integrations, AI and Automation are extension modules.
- Disabled extensions do not appear in primary navigation, search or quick actions.
- Enabled extensions appear inside the relevant primary area or under Settings → Active extensions; they do not create an uncontrolled top-level menu.
- Technical pages remain routable for backward compatibility but have no first-level navigation entry.

## Weekly-use test

Before promoting a feature to primary navigation, it must be used by the median activated business at least weekly and be required for one of the five core jobs. Otherwise it remains contextual, nested or modular.

## First-session sequence

Login → Overview → onboarding checklist → add branch → invite staff → activate NFC → test registration → record first visit. Empty states expose only the next useful action. Advanced configuration is not shown during activation.
