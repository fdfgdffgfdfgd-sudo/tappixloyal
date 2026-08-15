"use client";
import { useEffect, useState } from "react";
import { Check, CreditCard, LockKeyhole } from "lucide-react";
import { api } from "@/lib/api";
import { subscriptionStatusLabel } from "@/lib/labels";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Notice } from "./management-shared";

type Subscription = {
  plan: string;
  tier?: string;
  status: string;
  amount: number;
  currency: string;
  billingPeriod?: string;
  currentPeriodEndsAt?: string;
  modules?: string[];
  entitlements?: Record<string, { enabled: boolean; limit?: number | null }>;
};
type SubscriptionModule = { code:string; name:string; enabled:boolean; available:boolean; requiredPlan:string };
export function SubscriptionPage() {
  const [value, setValue] = useState<Subscription | null>(null);
  const [modules, setModules] = useState<SubscriptionModule[]>([]);
  const [msg, setMsg] = useState("");
  useEffect(() => {
    Promise.all([api<Subscription>("/subscription"),api<SubscriptionModule[]>("/modules")])
      .then(([subscription,moduleItems])=>{setValue(subscription);setModules(moduleItems)})
      .catch((e) => setMsg(e.message));
  }, []);
  return (
    <SectionShell
      active="/subscription"
      title="Подписка"
      subtitle="Тариф и доступные возможности"
    >
      <Notice text={msg} />
      {value && (
        <>
          <div className="subscription-card current-plan-card">
            <span className="entity-icon">
              <CreditCard />
            </span>
            <p>Текущий тариф</p>
            <h2>{value.plan}</h2>
            <strong>
              {value.amount.toLocaleString("ru-RU")} {value.currency} / месяц
            </strong>
            <div className="subscription-meta">
              <span>
                Статус <b>{subscriptionStatusLabel(value.status)}</b>
              </span>
              <span>
                Следующее продление{" "}
                <b>
                  {value.currentPeriodEndsAt
                    ? new Date(value.currentPeriodEndsAt).toLocaleDateString(
                        "ru-RU",
                      )
                    : "Пробный период"}
                </b>
              </span>
            </div>
            <div className="subscription-live-state">
              <div><Check/><span><b>{modules.filter(module=>module.enabled).length} функций активировано</b><small>Доступ пересчитывается сервером по текущему тарифу</small></span></div>
              <Link href="/modules">Все возможности</Link>
            </div>
            <a className="primary-action" href="#plans">
              Сравнить тарифы
            </a>
          </div>
          <section className="subscription-capabilities">
            <header><div><span>ДОСТУП ПРЯМО СЕЙЧАС</span><h2>Что работает на вашем тарифе</h2><p>«Активно» означает, что модуль включён. Для внешнего сервиса, например Poster, всё равно потребуется токен подключения.</p></div><Link href="/integrations">Открыть интеграции</Link></header>
            <div>{modules.map(module=><article className={module.enabled?"enabled":module.available?"available":"locked"} key={module.code}><span>{module.enabled?<Check/>:<LockKeyhole/>}</span><div><strong>{module.name}</strong><small>{module.enabled?"Готово к работе":module.available?"Доступно — требуется настройка":`Доступно с тарифа ${module.requiredPlan}`}</small></div><b>{module.enabled?"Активно":module.available?"Настроить":module.requiredPlan}</b></article>)}</div>
          </section>
          <section className="pricing-section" id="plans">
            <div className="pricing-heading">
              <span>ТАРИФЫ TAPPIX</span>
              <h2>Выберите возможности под ваш рост</h2>
              <p>
                Ваш тариф подсвечен. Переход оформляется через Tappix, поэтому
                функции нельзя включить в обход подписки.
              </p>
            </div>
            <div className="pricing-grid">
              {[
                {
                  name: "Starter",
                  price: 19900,
                  desc: "Для запуска одной программы",
                  features: [
                    "До 500 клиентов",
                    "2 сотрудника",
                    "2 NFC/QR точки",
                    "CRM и полная история визитов",
                    "Базовая детальная аналитика",
                    "Бонусы, штампы, награды и отзывы",
                    "Дизайн Guest Portal",
                  ],
                },
                {
                  name: "Growth",
                  price: 49900,
                  desc: "Для растущего бизнеса",
                  features: [
                    "До 5 000 клиентов",
                    "10 сотрудников",
                    "20 NFC/QR точек",
                    "Всё из Starter",
                    "Расширенные сегменты и удержание",
                    "До 2 000 сообщений в месяц",
                    "Автоматические возвратные кампании",
                    "Онлайн-запись и Mini Site",
                  ],
                },
                {
                  name: "Pro",
                  price: 99900,
                  desc: "Для сети и интеграций",
                  features: [
                    "До 50 000 клиентов",
                    "50 сотрудников",
                    "200 NFC/QR точек",
                    "Всё из Growth",
                    "API и webhooks",
                    "Сводная аналитика сети",
                    "Собственный домен и приоритетная поддержка",
                    "Индивидуальные лимиты и интеграции",
                  ],
                },
              ].map((plan) => {
                const current =
                  value.plan.toLowerCase() ===
                    (plan.name === "Growth"
                      ? "business"
                      : plan.name.toLowerCase()) ||
                  value.plan.toLowerCase() === plan.name.toLowerCase();
                return (
                  <article
                    className={
                      current ? "pricing-card current" : "pricing-card"
                    }
                    key={plan.name}
                  >
                    {current && <b className="current-ribbon">Ваш тариф</b>}
                    <h3>{plan.name}</h3>
                    <p>{plan.desc}</p>
                    <strong>
                      {plan.price.toLocaleString("ru-RU")} ₸{" "}
                      <small>/ месяц</small>
                    </strong>
                    <ul>
                      {plan.features.map((x) => (
                        <li key={x}>
                          <Check />
                          {x}
                        </li>
                      ))}
                    </ul>
                    <button disabled={current}>
                      {current ? "Подключён" : "Оставить заявку"}
                    </button>
                  </article>
                );
              })}
            </div>
          </section>
        </>
      )}
    </SectionShell>
  );
}
