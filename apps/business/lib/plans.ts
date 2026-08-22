export type PlanDefinition = {
  id: "starter" | "growth" | "pro";
  name: string;
  monthlyPrice: number;
  annualPrice: number;
  currency: string;
  trialDays: number;
  description: string;
  highlighted: boolean;
};

export const planPresentation = {
  starter: {
    locations: "1 филиал",
    features: ["CRM и клиентская база", "QR loyalty и Staff Mode", "Штампы, бонусы и награды", "Базовая аналитика", "Email-кампании"],
  },
  growth: {
    locations: "1–2 филиала",
    features: ["Всё из Start", "Сегменты и удержание", "Автоматические кампании", "Poster и WhatsApp", "Расширенные отчёты и экспорт"],
  },
  pro: {
    locations: "До 5 филиалов",
    features: ["Всё из Pro", "Аналитика по филиалам", "Расширенные роли", "API и webhooks", "Увеличенные лимиты"],
  },
} as const;

export const fallbackPlans: PlanDefinition[] = [
  { id: "starter", name: "Start", monthlyPrice: 7990, annualPrice: 79900, currency: "KZT", trialDays: 7, description: "Для малого бизнеса и первой точки", highlighted: false },
  { id: "growth", name: "Pro", monthlyPrice: 14990, annualPrice: 149900, currency: "KZT", trialDays: 7, description: "Для растущего бизнеса и автоматизации", highlighted: true },
  { id: "pro", name: "Business", monthlyPrice: 24990, annualPrice: 249900, currency: "KZT", trialDays: 7, description: "Для сетей и нескольких филиалов", highlighted: false },
];
