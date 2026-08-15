"use client";
import { useEffect, useState } from "react";
import { Check, Gift, LockKeyhole, ShieldCheck } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Module, Notice } from "./management-shared";

export function ModulesPage() {
  const [items, setItems] = useState<Module[]>([]);
  const [msg, setMsg] = useState("");
  const load = () =>
    api<Module[]>("/modules")
      .then(setItems)
      .catch((e) => setMsg(e.message));
  useEffect(() => {
    void load();
  }, []);
  const planAccess: Record<string, string> = {
    core: "Starter",
    crm: "Starter",
    loyalty: "Starter",
    reviews: "Starter",
    analytics: "Growth",
    email: "Growth",
    sms: "Growth",
    telegram: "Growth",
    booking: "Growth",
    website: "Growth",
    api: "Pro",
  };
  return (
    <SectionShell
      active="/modules"
      title="Модули"
      subtitle="Что входит в ваш тариф и какие функции можно подключить"
    >
      <Notice text={msg} />
      <div className="module-grid">
        {items.map((item) => {
          const included = item.core || item.enabled;
          return (
            <article
              className={`module-card ${included ? "module-included" : "module-locked"}`}
              key={item.code}
            >
              <span className="entity-icon">
                <Gift />
              </span>
              <div>
                <h2>{item.name}</h2>
                <p>
                  {included
                    ? "Доступно в вашей подписке"
                    : `Доступно начиная с тарифа ${planAccess[item.code] || "Pro"}`}
                </p>
              </div>
              <span className="module-access-state">
                {included ? (
                  <>
                    <Check />
                    Подключено
                  </>
                ) : (
                  <>
                    <LockKeyhole />
                    Недоступно
                  </>
                )}
              </span>
            </article>
          );
        })}
      </div>
      <div className="module-explainer">
        <ShieldCheck />
        <div>
          <strong>Почему здесь нет переключателей?</strong>
          <p>
            Владелец не может сам выдать себе платную функцию. Доступ
            определяется тарифом, а индивидуальные исключения задаёт только
            Tappix Platform. Так биллинг и права всегда совпадают.
          </p>
        </div>
        <Link href="/subscription">Сравнить тарифы</Link>
      </div>
    </SectionShell>
  );
}
