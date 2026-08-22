"use client";
import { useEffect, useState } from "react";
import { Check, CreditCard, LockKeyhole } from "lucide-react";
import { api } from "@/lib/api";
import { subscriptionStatusLabel } from "@/lib/labels";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Notice } from "./management-shared";
import { API_URL } from "@/lib/api";
import { fallbackPlans, planPresentation, type PlanDefinition } from "@/lib/plans";

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
  const [plans, setPlans] = useState<PlanDefinition[]>(fallbackPlans);
  const [annual, setAnnual] = useState(false);
  const [msg, setMsg] = useState("");
  useEffect(() => {
    Promise.all([api<Subscription>("/subscription"),api<SubscriptionModule[]>("/modules")])
      .then(([subscription,moduleItems])=>{setValue(subscription);setAnnual(subscription.billingPeriod==="annual");setModules(moduleItems)})
      .catch((e) => setMsg(e.message));
  }, []);
  const currentPlan = value ? plans.find(plan => plan.id === (value.plan.toLowerCase()==="business"?"growth":value.plan.toLowerCase()==="enterprise"?"pro":value.plan.toLowerCase())) : undefined;
  useEffect(() => {
    fetch(`${API_URL}/public/plans`).then(response => response.json()).then(result => result.success && setPlans(result.data)).catch(() => {});
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
              {(value.billingPeriod==="annual"&&currentPlan?currentPlan.annualPrice:value.amount).toLocaleString("ru-RU")} {value.currency} / {value.billingPeriod==="annual"?"год":"месяц"}
            </strong>
            <div className="subscription-meta">
              <span>
                Статус <b>{subscriptionStatusLabel(value.status)}</b>
              </span>
              <span>
                Период <b>{value.billingPeriod==="annual"?"Годовой":"Ежемесячный"}</b>
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
                Единые цены для сайта и Billing. Годовая оплата экономит стоимость двух месяцев.
              </p>
              <div className="billing-switch" role="group" aria-label="Период оплаты"><button className={!annual ? "active" : ""} onClick={() => setAnnual(false)}>Ежемесячно</button><button className={annual ? "active" : ""} onClick={() => setAnnual(true)}>За год <small>2 месяца бесплатно</small></button></div>
            </div>
            <div className="pricing-grid">
              {plans.map((plan) => {
                const current =
                  value.plan.toLowerCase() ===
                    (plan.id === "growth"
                      ? "business"
                      : plan.id === "pro" ? "enterprise" : plan.id) ||
                  value.plan.toLowerCase() === plan.id;
                const presentation = planPresentation[plan.id];
                return (
                  <article
                    className={
                      current ? "pricing-card current" : "pricing-card"
                    }
                    key={plan.name}
                  >
                    {current && <b className="current-ribbon">Ваш тариф</b>}
                    <h3>{plan.name}</h3>
                    <p>{plan.description}</p>
                    <strong>
                      {(annual ? plan.annualPrice : plan.monthlyPrice).toLocaleString("ru-RU")} ₸{" "}
                      <small>/ {annual ? "год" : "месяц"}</small>
                    </strong>
                    {annual&&<p className="effective-monthly">Эквивалент {(plan.annualPrice/12).toLocaleString("ru-RU",{maximumFractionDigits:0})} ₸ в месяц</p>}
                    <ul>
                      {presentation.features.map((x) => (
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
