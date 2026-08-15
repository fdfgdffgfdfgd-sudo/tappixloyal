"use client";
import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Clock3, Gift, ShieldCheck, ShieldAlert } from "lucide-react";
import { api } from "@/lib/api";
import { customerLevelLabel } from "@/lib/labels";
import { SectionShell } from "./section-shell";
import { Customer, Branch, Notice } from "./management-shared";
import { useConfirm } from "./use-confirm";

type CustomerHistory = {
  bonuses: {
    id: string;
    operation: string;
    amount: number;
    balanceAfter: number;
    description: string;
    createdAt: string;
  }[];
  visits: {
    id: string;
    pointsAdded: number;
    comment: string;
    createdAt: string;
    branch: string;
    employee: string;
  }[];
};
type CustomerReward = {
  id: string;
  name: string;
  description: string;
  status: string;
  issuedAt: string;
  expiresAt?: string;
  redeemedAt?: string;
};
type CustomerTimelineEvent = {
  id: string;
  type: string;
  occurredAt: string;
  branch?: string;
  properties: Record<string, string | number | boolean>;
};
type CustomerRisk = {id:string;operation:string;severity:"warning"|"blocked";status:string;reason:string;createdAt:string;branch?:string;actor?:string};
export function CustomerDetailPage({ id }: { id: string }) {
  const { ask, dialog } = useConfirm();
  const router = useRouter();
  const [now] = useState(() => Date.now());
  const [value, setValue] = useState<Customer | null>(null);
  const [history, setHistory] = useState<CustomerHistory>({
    bonuses: [],
    visits: [],
  });
  const [branches, setBranches] = useState<Branch[]>([]);
  const [rewards, setRewards] = useState<CustomerReward[]>([]);
  const [timeline, setTimeline] = useState<CustomerTimelineEvent[]>([]);
  const [risks, setRisks] = useState<CustomerRisk[]>([]);
  const [msg, setMsg] = useState("");
  const [bonus, setBonus] = useState<"credit" | "debit" | null>(null);
  const load = async () => {
    try {
      const [c, h, b, g, events, riskItems] = await Promise.all([
        api<Customer>(`/customers/${id}`),
        api<CustomerHistory>(`/customers/${id}/history`),
        api<Branch[]>("/branches"),
        api<CustomerReward[]>(`/customers/${id}/rewards`),
        api<CustomerTimelineEvent[]>(`/customers/${id}/timeline`),
        api<CustomerRisk[]>(`/customers/${id}/risk`),
      ]);
      setValue(c);
      setHistory(h);
      setBranches(b);
      setRewards(g);
      setTimeline(events);
      setRisks(riskItems);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  };
  useEffect(() => {
    void load();
  }, [id]);
  async function save(e: FormEvent) {
    e.preventDefault();
    if (!value) return;
    try {
      await api(`/customers/${id}`, {
        method: "PATCH",
        body: JSON.stringify({
          firstName: value.firstName,
          lastName: value.lastName,
          phone: value.phone,
          email: value.email || "",
          birthday: value.birthday?.slice(0, 10) || "",
          level: value.level,
        }),
      });
      setMsg("Карточка клиента сохранена");
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  }
  async function archive() {
    if (!await ask({ title: "Архивировать клиента?", confirmLabel: "Архивировать" })) return;
    await api(`/customers/${id}`, { method: "DELETE" });
    router.push("/customers");
  }
  async function bonusSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = Object.fromEntries(new FormData(e.currentTarget));
    try {
      await api(`/customers/${id}/bonus`, {
        method: "POST",
        body: JSON.stringify({
          ...data,
          operation: bonus,
          amount: Number(data.amount),
        }),
      });
      setBonus(null);
      setMsg(bonus === "credit" ? "Бонусы начислены" : "Бонусы списаны");
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  }
  async function visit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const data = Object.fromEntries(new FormData(e.currentTarget));
    try {
      const result = await api<{ reward?: string }>("/visits", {
        method: "POST",
        body: JSON.stringify({ customerId: id, ...data }),
      });
      setMsg(
        result.reward
          ? `Посещение добавлено · выдан подарок «${result.reward}»`
          : "Посещение добавлено",
      );
      e.currentTarget.reset();
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  }
  async function redeemReward(reward: CustomerReward) {
    if (!confirm(`Отметить подарок «${reward.name}» использованным?`)) return;
    try {
      await api(`/rewards/${reward.id}`, {
        method: "PATCH",
        body: JSON.stringify({ status: "redeemed" }),
      });
      setMsg("Подарок отмечен использованным");
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  }
  if (!value)
    return (
      <SectionShell
        active="/customers"
        title="Карточка клиента"
        subtitle="Загрузка…"
      >
        <Notice text={msg} />
      {dialog}</SectionShell>
    );
  const visitDates = history.visits
    .map((visit) => new Date(visit.createdAt).getTime())
    .sort((a, b) => b - a);
  const daysSinceVisit = visitDates[0]
    ? Math.floor((now - visitDates[0]) / 86400000)
    : null;
  const averageInterval =
    visitDates.length > 1
      ? Math.round(
          visitDates
            .slice(0, -1)
            .reduce(
              (sum, date, index) =>
                sum + Math.abs(date - visitDates[index + 1]),
              0,
            ) /
            (visitDates.length - 1) /
            86400000,
        )
      : null;
  const customerSegment =
    value.totalVisits >= 10
      ? "Постоянный клиент"
      : value.totalVisits >= 5
        ? "Частый гость"
        : daysSinceVisit !== null && daysSinceVisit > 45
          ? "Риск ухода"
          : value.totalVisits > 0
            ? "Развивающийся"
            : "Новый клиент";
  return (
    <SectionShell
      active="/customers"
      title={`${value.firstName} ${value.lastName}`}
      subtitle={value.phone}
    >
      <Notice text={msg} />
      <div className="customer-summary">
        <article>
          <span>Баланс</span>
          <strong>{value.totalPoints} б.</strong>
        </article>
        <article>
          <span>Посещения</span>
          <strong>{value.totalVisits}</strong>
        </article>
        <article>
          <span>Уровень</span>
          <strong>{customerLevelLabel(value.level)}</strong>
        </article>
        <article className="customer-segment-card">
          <span>Сегмент</span>
          <strong>{customerSegment}</strong>
          <small>
            {daysSinceVisit === null
              ? "Ещё не посещал"
              : `Последний визит ${daysSinceVisit} дн. назад`}
          </small>
        </article>
        <article>
          <span>Средняя частота</span>
          <strong>
            {averageInterval
              ? `раз в ${averageInterval} дн.`
              : "Недостаточно данных"}
          </strong>
          <small>
            {value.totalVisits >= 5
              ? "Активность выше базовой"
              : "Формируется после 2 визитов"}
          </small>
        </article>
        <article>
          <span>Любимый филиал</span>
          <strong>{value.favoriteBranch||"Пока не определён"}</strong>
          <small>{value.lastBranch?`Последний визит: ${value.lastBranch}`:"Появится после первого визита"}</small>
        </article>
      </div>
      {risks.some(item=>item.status==="open")&&<section className="customer-risk-callout" role="status"><ShieldAlert/><div><strong>Нужна проверка операций</strong><p>{risks.filter(item=>item.status==="open").length} подозрительных действий остановлено автоматически. Бонусы и посещения клиента не изменились.</p></div></section>}
      <div className="customer-actions">
        <button className="primary-action" onClick={() => setBonus("credit")}>
          Начислить бонусы
        </button>
        <button onClick={() => setBonus("debit")}>Списать бонусы</button>
        <button className="danger-action" onClick={archive}>
          Архивировать
        </button>
      </div>
      <div className="detail-grid">
        <form className="settings-card" onSubmit={save}>
          <h2>Профиль</h2>
          <div className="form-grid">
            <label>
              Имя
              <input
                value={value.firstName}
                onChange={(e) =>
                  setValue({ ...value, firstName: e.target.value })
                }
              />
            </label>
            <label>
              Фамилия
              <input
                value={value.lastName}
                onChange={(e) =>
                  setValue({ ...value, lastName: e.target.value })
                }
              />
            </label>
            <label>
              Телефон
              <input
                value={value.phone}
                onChange={(e) => setValue({ ...value, phone: e.target.value })}
              />
            </label>
            <label>
              Email
              <input
                type="email"
                value={value.email || ""}
                onChange={(e) => setValue({ ...value, email: e.target.value })}
              />
            </label>
            <label>
              Дата рождения
              <input
                type="date"
                value={value.birthday?.slice(0, 10) || ""}
                onChange={(e) =>
                  setValue({ ...value, birthday: e.target.value })
                }
              />
            </label>
            <label>
              Уровень
              <select
                value={value.level}
                onChange={(e) => setValue({ ...value, level: e.target.value })}
              >
                <option value="basic">Базовый</option>
                <option value="silver">Серебряный</option>
                <option value="gold">Золотой</option>
                <option value="vip">VIP</option>
              </select>
            </label>
          </div>
          <button className="primary-action">Сохранить</button>
        </form>
        <form className="settings-card visit-form" onSubmit={visit}>
          <h2>Добавить посещение</h2>
          <label>
            Филиал
            <select name="branchId" required>
              {branches.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Комментарий
            <textarea name="comment" rows={3} />
          </label>
          <button className="primary-action">Добавить посещение</button>
        </form>
      </div>
      <div className="detail-grid history-grid">
        <section className="data-card reward-admin-list">
          <h2>Подарки</h2>
          {rewards.map((x) => (
            <article className="history-line" key={x.id}>
              <b className={x.status === "available" ? "positive" : "negative"}>
                <Gift />
              </b>
              <div>
                <strong>{x.name}</strong>
                <small>
                  {x.status === "available"
                    ? `Доступен${x.expiresAt ? ` до ${new Date(x.expiresAt).toLocaleDateString("ru-RU")}` : ""}`
                    : x.status === "redeemed"
                      ? "Использован"
                      : "Недоступен"}
                </small>
              </div>
              {x.status === "available" && (
                <button onClick={() => redeemReward(x)}>Погасить</button>
              )}
            </article>
          ))}
          {!rewards.length && (
            <div className="zero">
              <Gift />
              <strong>Подарков пока нет</strong>
              <p>Они появятся после выполнения правила посещений.</p>
            </div>
          )}
        </section>
        <section className="data-card customer-event-timeline">
          <header><div><h2>История клиента</h2><p>Все важные события в одной ленте</p></div><Clock3/></header>
          {timeline.map((event) => {
            const labels: Record<string,string> = {"customer.registered":"Зарегистрировался через NFC или QR","visit.completed":"Посещение","purchase.completed":"Покупка","bonus.earned":"Бонусы начислены","bonus.spent":"Бонусы списаны","reward.almost_unlocked":"До награды остался один визит","reward.unlocked":"Награда стала доступна","reward.redeemed":"Награда использована","reward.expired":"Срок награды истёк","referral.created":"Поделился приглашением","referral.converted":"Друг совершил покупку","campaign.sent":"Получил сообщение","campaign.opened":"Открыл сообщение","customer.returned":"Вернулся"};
            const amount=Number(event.properties.amount||event.properties.pointsAdded||0);
            const detail=event.properties.reason||event.properties.name;
            const netAmount=Number(event.properties.netAmount||0);
            return <article key={event.id}><span><Clock3/></span><div><strong>{labels[event.type]||event.type}</strong><small>{new Date(event.occurredAt).toLocaleString("ru-RU")}{event.branch?` · ${event.branch}`:""}</small>{detail&&<p>{String(detail)}</p>}</div>{amount>0?<b className={event.type==="bonus.spent"?"spent":""}>{event.type==="bonus.spent"?"−":"+"}{amount}</b>:netAmount>0?<b>{Math.round(netAmount).toLocaleString("ru-RU")} ₸</b>:null}</article>;
          })}
          {!timeline.length&&<div className="zero"><Clock3/><strong>История начнётся с первого действия</strong><p>Регистрация, посещения, покупки, бонусы и награды появятся здесь по времени.</p></div>}
        </section>
        <section className="data-card customer-risk-list">
          <header><div><h2>Контроль операций</h2><p>Автоматически остановленные повторы и превышения лимитов</p></div><ShieldAlert/></header>
          {risks.map(item=><article key={item.id}><span><ShieldAlert/></span><div><strong>{item.reason}</strong><small>{new Date(item.createdAt).toLocaleString("ru-RU")}{item.branch?` · ${item.branch}`:""}{item.actor?` · ${item.actor}`:""}</small></div><b>{item.severity==="blocked"?"Остановлено":"Проверить"}</b></article>)}
          {!risks.length&&<div className="zero"><ShieldCheck/><strong>Подозрительных операций нет</strong><p>Повторы, частые посещения и крупные ручные списания контролируются автоматически.</p></div>}
        </section>
        <section className="data-card">
          <h2>Бонусная история</h2>
          {history.bonuses.map((x) => (
            <article className="history-line" key={x.id}>
              <b className={x.operation === "credit" ? "positive" : "negative"}>
                {x.operation === "credit" ? "+" : "−"}
                {x.amount}
              </b>
              <div>
                <strong>{x.description}</strong>
                <small>{new Date(x.createdAt).toLocaleString("ru-RU")}</small>
              </div>
              <span>{x.balanceAfter} б.</span>
            </article>
          ))}
        </section>
        <section className="data-card">
          <h2>Посещения</h2>
          {history.visits.map((x) => (
            <article className="history-line" key={x.id}>
              <b className="positive">+{x.pointsAdded}</b>
              <div>
                <strong>{x.branch}</strong>
                <small>
                  {new Date(x.createdAt).toLocaleString("ru-RU")} · {x.employee}
                </small>
              </div>
            </article>
          ))}
        </section>
      </div>
      {bonus && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={bonusSubmit}>
            <h2>{bonus === "credit" ? "Начислить" : "Списать"} бонусы</h2>
            <label>
              Количество
              <input name="amount" type="number" min="1" required />
            </label>
            <label>
              Описание
              <input
                name="description"
                required
                placeholder="Причина операции"
              />
            </label>
            <div>
              <button type="button" onClick={() => setBonus(null)}>
                Отмена
              </button>
              <button className="primary-action">Подтвердить</button>
            </div>
          </form>
        </div>
      )}
    {dialog}</SectionShell>
  );
}
