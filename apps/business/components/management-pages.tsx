"use client";
import { FormEvent, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  BarChart3,
  AlertTriangle,
  Banknote,
  Bell,
  Building2,
  Cake,
  Check,
  CreditCard,
  Crown,
  Clock3,
  FileDown,
  Gift,
  LockKeyhole,
  Nfc,
  Pencil,
  Plus,
  QrCode,
  RefreshCw,
  Search,
  Repeat2,
  Send,
  ShieldCheck,
  ShieldAlert,
  Star,
  ToggleLeft,
  ToggleRight,
  Trash2,
  UserX,
  UserCheck,
  TrendingUp,
  Users,
} from "lucide-react";
import { RewardBuilder } from "./reward-builder";
import { ProgramMechanics } from "./program-mechanics";
export { NFCQRManager as DevicesPage } from "./nfc-qr-manager";
import { api, download } from "@/lib/api";
import { customerLevelLabel, subscriptionStatusLabel } from "@/lib/labels";
import { SectionShell } from "./section-shell";

type Customer = {
  id: string;
  firstName: string;
  lastName: string;
  phone: string;
  email?: string;
  birthday?: string;
  totalVisits: number;
  totalPoints: number;
  level: string;
  createdAt: string;
  favoriteBranch?: string;
  lastBranch?: string;
};
type Branch = {
  id: string;
  name: string;
  address: string;
  active: boolean;
  phone?: string;
};
type Module = { code: string; name: string; core: boolean; enabled: boolean };
type Loyalty = {
  welcomeBonus: number;
  pointsPerVisit: number;
  birthdayBonus: number;
  visitsForReward: number;
  rewardName: string;
};
function Notice({ text }: { text: string }) {
  return text ? (
    <div className="notice" role="status">
      <Check size={17} />
      {text}
    </div>
  ) : null;
}

export function CustomersPage() {
  const [items, setItems] = useState<Customer[]>([]);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [q, setQ] = useState("");
  const [searchQ, setSearchQ] = useState("");
  const [level, setLevel] = useState("");
  const [branch, setBranch] = useState("");
  const [birthday, setBirthday] = useState("");
  const [minPoints, setMinPoints] = useState("");
  const [sort, setSort] = useState("createdAt");
  const [order, setOrder] = useState("desc");
  const [page, setPage] = useState(1);
  const [pages, setPages] = useState(1);
  const [total, setTotal] = useState(0);
  const [open, setOpen] = useState(false);
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [filtersReady, setFiltersReady] = useState(false);
  const query = new URLSearchParams({
    search: searchQ,
    level,
    branch,
    birthday,
    minPoints,
    sort,
    order,
  }).toString();
  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const x = await api<{ items: Customer[]; pages: number; total: number }>(`/customers?limit=20&page=${page}&${query}`);
        setItems(x.items);
        setPages(Math.max(1, x.pages));
        setTotal(x.total);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Не удалось загрузить клиентов");
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    api<Branch[]>("/branches")
      .then(setBranches)
      .catch(() => {});
    try {
      const saved = JSON.parse(sessionStorage.getItem("tappix_customer_filters") || "{}");
      setQ(saved.q || ""); setSearchQ(saved.q || ""); setLevel(saved.level || ""); setBranch(saved.branch || ""); setBirthday(saved.birthday || ""); setMinPoints(saved.minPoints || ""); setSort(saved.sort || "createdAt"); setOrder(saved.order || "desc"); setPage(Number(saved.page) || 1);
    } catch { /* ignore invalid saved filters */ }
    setFiltersReady(true);
  }, []);
  useEffect(() => {
    const timer = window.setTimeout(() => setSearchQ(q.trim()), 300);
    return () => window.clearTimeout(timer);
  }, [q]);
  useEffect(() => {
    if (!filtersReady) return;
    void load();
    sessionStorage.setItem("tappix_customer_filters", JSON.stringify({q:searchQ,level,branch,birthday,minPoints,sort,order,page}));
  }, [filtersReady, searchQ, level, branch, birthday, minPoints, sort, order, page]);
  useEffect(
    () => setPage(1),
    [searchQ, level, branch, birthday, minPoints, sort, order],
  );
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const payload = Object.fromEntries(form);
    const digits = String(payload.phone || "").replace(/\D/g, "");
    payload.phone = digits.length === 11 && digits.startsWith("8") ? `+7${digits.slice(1)}` : digits.length === 11 ? `+${digits}` : String(payload.phone || "").trim();
    setSaving(true);
    try {
      await api("/customers", {
        method: "POST",
        body: JSON.stringify(payload),
      });
      setOpen(false);
      setMsg("Клиент создан");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    } finally { setSaving(false); }
  }
  async function exportCSV() {
    try {
      await download(`/customers/export?${query}`, "tappix-customers.csv");
      setMsg("Экспорт клиентов подготовлен");
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  function resetFilters() {
    setQ("");
    setLevel("");
    setBranch("");
    setBirthday("");
    setMinPoints("");
    setSort("createdAt");
    setOrder("desc");
  }
  return (
    <SectionShell
      active="/customers"
      title="Клиенты"
      subtitle="CRM вашей компании"
    >
      <Notice text={msg} />
      {error && <div className="crm-error" role="alert"><span><AlertTriangle/><strong>Не удалось загрузить клиентов</strong><small>{error}</small></span><button onClick={() => void load()}><RefreshCw/>Повторить</button></div>}
      <div className="toolbar crm-toolbar">
        <label className="searchbox">
          <Search />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Имя или телефон"
          />
        </label>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Plus />
          Новый клиент
        </button>
      </div>
      <details className="crm-filter-panel">
        <summary><span><Search/>Фильтры и сортировка</span><small>Уровень, филиал, день рождения и баланс</small></summary>
      <div className="crm-filters" aria-label="Фильтры клиентов">
        <label>
          Уровень
          <select value={level} onChange={(e) => setLevel(e.target.value)}>
            <option value="">Все уровни</option>
            <option value="basic">Базовый</option>
            <option value="silver">Серебряный</option>
            <option value="gold">Золотой</option>
            <option value="vip">VIP</option>
          </select>
        </label>
        <label>
          Филиал
          <select value={branch} onChange={(e) => setBranch(e.target.value)}>
            <option value="">Все филиалы</option>
            {branches.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          День рождения
          <select
            value={birthday}
            onChange={(e) => setBirthday(e.target.value)}
          >
            <option value="">Любая дата</option>
            <option value="today">Сегодня</option>
            <option value="month">В этом месяце</option>
          </select>
        </label>
        <label>
          Бонусов от
          <input
            type="number"
            min="0"
            value={minPoints}
            onChange={(e) => setMinPoints(e.target.value)}
            placeholder="0"
          />
        </label>
        <label>
          Сортировка
          <select
            value={`${sort}:${order}`}
            onChange={(e) => {
              const [v, d] = e.target.value.split(":");
              setSort(v);
              setOrder(d);
            }}
          >
            <option value="createdAt:desc">Сначала новые</option>
            <option value="createdAt:asc">Сначала старые</option>
            <option value="name:asc">По имени</option>
            <option value="points:desc">Больше бонусов</option>
            <option value="visits:desc">Больше посещений</option>
          </select>
        </label>
        <button onClick={resetFilters}>Сбросить</button>
        <button onClick={exportCSV}><FileDown/>Экспорт CSV</button>
      </div>
      </details>
      <div className={`data-card ${loading ? "is-loading" : ""}`} aria-busy={loading}>
        <table>
          <thead>
            <tr>
              <th>Клиент</th>
              <th>Телефон</th>
              <th>Уровень</th>
              <th>Сегмент</th>
              <th>Посещения</th>
              <th>Баланс</th>
            </tr>
          </thead>
          <tbody>
            {!loading && items.map((c) => (
              <tr key={c.id}>
                <td data-label="Клиент">
                  <Link className="customer-link" href={`/customers/${c.id}`}>
                    <strong>
                      {c.firstName} {c.lastName}
                    </strong>
                  </Link>
                </td>
                <td data-label="Телефон">{c.phone}</td>
                <td data-label="Уровень">
                  <span className="tag">{customerLevelLabel(c.level)}</span>
                </td>
                <td data-label="Сегмент">
                  <span
                    className={`customer-segment segment-${c.totalVisits >= 10 ? "loyal" : c.totalVisits >= 5 ? "frequent" : "new"}`}
                  >
                    {c.totalVisits >= 10
                      ? "Постоянный"
                      : c.totalVisits >= 5
                        ? "Частый"
                        : "Новый"}
                  </span>
                </td>
                <td data-label="Посещения">{c.totalVisits}</td>
                <td data-label="Баланс">
                  <b>{c.totalPoints} б.</b>
                </td>
              </tr>
            ))}
            {loading && Array.from({length:5},(_,index)=><tr className="crm-skeleton" key={index}>{Array.from({length:6},(_,cell)=><td key={cell}><span/></td>)}</tr>)}
          </tbody>
        </table>
        {!loading && !error && !items.length && (
          <div className="zero">
            <Users />
            <strong>Клиенты не найдены</strong>
            <p>Измените фильтры или создайте нового клиента.</p>
          </div>
        )}
      </div>
      {!error && <nav className="crm-pagination" aria-label="Пагинация клиентов">
        <span>Найдено: {total}</span>
        <div>
          <button disabled={page <= 1} onClick={() => setPage((x) => x - 1)}>
            Назад
          </button>
          <b>
            {page} / {pages}
          </b>
          <button
            disabled={page >= pages}
            onClick={() => setPage((x) => x + 1)}
          >
            Далее
          </button>
        </div>
      </nav>}
      {open && (
        <div className="sheet-bg" onMouseDown={event => { if(event.target===event.currentTarget&&!saving)setOpen(false) }}>
          <form className="sheet" onSubmit={create}>
            <h2>Новый клиент</h2>
            <label>
              Имя
              <input name="firstName" autoComplete="given-name" required autoFocus />
            </label>
            <label>
              Фамилия
              <input name="lastName" autoComplete="family-name" />
            </label>
            <label>
              Телефон
              <input name="phone" type="tel" inputMode="tel" autoComplete="tel" placeholder="+7 700 000 00 00" required />
            </label>
            <label>
              Email
              <input name="email" type="email" autoComplete="email" />
            </label>
            <label>
              Дата рождения
              <input name="birthday" type="date" />
            </label>
            <div>
              <button type="button" disabled={saving} onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button className="primary-action" disabled={saving}>{saving?"Создаём…":"Создать"}</button>
            </div>
          </form>
        </div>
      )}
    </SectionShell>
  );
}

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
    if (!confirm("Архивировать клиента?")) return;
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
      </SectionShell>
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
    </SectionShell>
  );
}

export function BranchesPage() {
  const [items, setItems] = useState<Branch[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Branch | null>(null);
  const [msg, setMsg] = useState("");
  const load = () =>
    api<Branch[]>("/branches")
      .then(setItems)
      .catch((e) => setMsg(e.message));
  useEffect(() => {
    void load();
  }, []);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    try {
      await api("/branches", {
        method: "POST",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setOpen(false);
      setMsg("Филиал добавлен");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    try {
      await api(`/branches/${editing.id}`, {
        method: "PATCH",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setEditing(null);
      setMsg("Филиал обновлён");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function remove(id: string) {
    if (!confirm("Архивировать филиал?")) return;
    try {
      await api(`/branches/${id}`, { method: "DELETE" });
      setMsg("Филиал перенесён в архив");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  return (
    <SectionShell
      active="/branches"
      title="Филиалы"
      subtitle="Точки обслуживания компании"
    >
      <Notice text={msg} />
      <div className="toolbar">
        <span>{items.length} филиал</span>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Plus />
          Добавить филиал
        </button>
      </div>
      <div className="card-grid">
        {items.map((b) => (
          <article className="entity-card" key={b.id}>
            <span className="entity-icon">
              <Building2 />
            </span>
            <div>
              <h2>
                <Link className="customer-link" href={`/branches/${b.id}`}>
                  {b.name}
                </Link>
              </h2>
              <p>{b.address}</p>
              {b.phone && <small>{b.phone}</small>}
              <span className="status">Активен</span>
            </div>
            <div className="row-actions">
              <button
                aria-label="Редактировать филиал"
                onClick={() => setEditing(b)}
              >
                <Pencil />
              </button>
              <button
                className="danger-icon"
                aria-label="Архивировать филиал"
                onClick={() => remove(b.id)}
              >
                <Trash2 />
              </button>
            </div>
          </article>
        ))}
      </div>
      {open && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={create}>
            <h2>Новый филиал</h2>
            <label>
              Название
              <input name="name" required />
            </label>
            <label>
              Адрес
              <input name="address" required />
            </label>
            <label>
              Телефон
              <input name="phone" type="tel" />
            </label>
            <div>
              <button type="button" onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button className="primary-action">Добавить</button>
            </div>
          </form>
        </div>
      )}
      {editing && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={save}>
            <h2>Редактировать филиал</h2>
            <label>
              Название
              <input name="name" defaultValue={editing.name} required />
            </label>
            <label>
              Адрес
              <input name="address" defaultValue={editing.address} required />
            </label>
            <label>
              Телефон
              <input name="phone" type="tel" defaultValue={editing.phone} />
            </label>
            <div>
              <button type="button" onClick={() => setEditing(null)}>
                Отмена
              </button>
              <button className="primary-action">Сохранить</button>
            </div>
          </form>
        </div>
      )}
    </SectionShell>
  );
}

export function LoyaltyPage() {
  const [value, setValue] = useState<Loyalty>({
    welcomeBonus: 0,
    pointsPerVisit: 20,
    birthdayBonus: 0,
    visitsForReward: 10,
    rewardName: "Подарок",
  });
  const [msg, setMsg] = useState("");
  const [inactiveDays, setInactiveDays] = useState(30);
  const [inactive, setInactive] = useState<
    {
      id: string;
      firstName: string;
      lastName: string;
      phone: string;
      lastVisitAt?: string;
    }[]
  >([]);
  const [processing, setProcessing] = useState(false);
  useEffect(() => {
    api<Loyalty>("/loyalty/rules")
      .then(setValue)
      .catch((e) => setMsg(e.message));
  }, []);
  async function save(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/loyalty/rules", {
        method: "PATCH",
        body: JSON.stringify(value),
      });
      setMsg("Правила лояльности сохранены");
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function loadInactive(days = inactiveDays) {
    try {
      const result = await api<{ items: typeof inactive }>(
        `/loyalty/inactive?days=${days}`,
      );
      setInactive(result.items);
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function runBirthdays() {
    setProcessing(true);
    try {
      const result = await api<{ processed: number }>(
        "/loyalty/process-birthdays",
        { method: "POST", body: "{}" },
      );
      setMsg(
        result.processed
          ? `Birthday-бонус начислен ${result.processed} клиентам`
          : "Все birthday-бонусы за сегодня уже обработаны",
      );
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    } finally {
      setProcessing(false);
    }
  }
  useEffect(() => {
    void loadInactive();
  }, []);
  const field = (key: keyof Loyalty, label: string, type = "number") => (
    <label>
      {label}
      <input
        type={type}
        value={value[key]}
        onChange={(e) =>
          setValue({
            ...value,
            [key]: type === "number" ? Number(e.target.value) : e.target.value,
          })
        }
      />
    </label>
  );
  return (
    <SectionShell
      active="/loyalty"
      title="Программа лояльности"
      subtitle="Настройте начисления и награды без разработчика"
    >
      <Notice text={msg} />
      <div className="loyalty-workspace-panel"><ProgramMechanics />
        <section className="loyalty-launch-path"><header><div><small>ПОСЛЕ ПУБЛИКАЦИИ</small><h2>Три шага до первых результатов</h2></div></header><div className="loyalty-next-actions">
          <Link href="/devices"><b>1</b><QrCode/><span><strong>Разместите QR или NFC</strong><small>Клиент откроет и сохранит карту</small></span></Link>
          <Link href="/scanner"><b>2</b><Nfc/><span><strong>Отмечайте покупки</strong><small>Сотрудник сканирует карту гостя</small></span></Link>
          <Link href="/analytics"><b>3</b><TrendingUp/><span><strong>Следите за возвратом</strong><small>Увидите повторные визиты и выручку</small></span></Link>
        </div></section>
      </div>
      <details className="loyalty-secondary-section"><summary><Gift/><span><strong>Дополнительные награды</strong><small>Скидки, услуги и подарки поверх основной программы</small></span><Plus/></summary><div><RewardBuilder /></div></details>
      <details className="loyalty-secondary-section"><summary><Bell/><span><strong>Дополнительные правила</strong><small>Приветственный бонус, день рождения и возврат клиентов</small></span><Plus/></summary><div><div className="workspace-explainer"><Bell/><div><small>АВТОМАТИЧЕСКИЕ ДЕЙСТВИЯ</small><h2>Система действует в нужный момент</h2><p>Нулевое значение отключает соответствующее начисление.</p></div></div>
      <form className="settings-card" onSubmit={save}>
        <div className="settings-title">
          <span>
            <Gift />
          </span>
          <div>
            <h2>Автоматические начисления</h2>
            <p>Применяются ко всем филиалам. Нулевое значение отключает начисление.</p>
          </div>
        </div>
        <div className="form-grid">
          {field("welcomeBonus", "После регистрации, бонусов")}
          {field("pointsPerVisit", "После посещения, бонусов")}
          {field("birthdayBonus", "На день рождения, бонусов")}
          {field("visitsForReward", "Цель по посещениям")}
          {field("rewardName", "Подарок за достижение цели", "text")}
        </div>
        <button className="primary-action">Сохранить правила</button>
      </form>
      <div className="loyalty-automation-grid">
        <section className="automation-card">
          <span>
            <Cake />
          </span>
          <div>
            <h2>День рождения</h2>
            <p>
              Система начисляет настроенный бонус один раз в год. Кнопка запускает безопасную ручную проверку прямо сейчас.
            </p>
          </div>
          <button
            disabled={processing || value.birthdayBonus <= 0}
            onClick={runBirthdays}
          >
            {processing ? "Проверяем…" : value.birthdayBonus <= 0 ? "Сначала задайте бонус выше" : "Проверить сейчас"}
          </button>
        </section>
        <section className="inactive-card">
          <div className="inactive-title">
            <span>
              <UserX />
            </span>
            <div>
              <h2>Пора вернуть</h2>
              <p>Клиенты без визитов за выбранный период</p>
            </div>
            <select
              aria-label="Период неактивности"
              value={inactiveDays}
              onChange={(e) => {
                const days = Number(e.target.value);
                setInactiveDays(days);
                void loadInactive(days);
              }}
            >
              <option value={30}>30 дней</option>
              <option value={60}>60 дней</option>
              <option value={90}>90 дней</option>
            </select>
          </div>
          <strong>{inactive.length}</strong>
          <small>клиентов не возвращались</small>
          <div className="inactive-preview">
            {inactive.slice(0, 5).map((x) => (
              <Link key={x.id} href={`/customers/${x.id}`}>
                <span>
                  {x.firstName} {x.lastName}
                </span>
                <small>
                  {x.lastVisitAt
                    ? new Date(x.lastVisitAt).toLocaleDateString("ru-RU")
                    : "Ещё не посещал"}{" "}
                  · {x.phone}
                </small>
              </Link>
            ))}
            {!inactive.length && <p>В этом сегменте сейчас нет клиентов.</p>}
          </div>
          {inactive.length > 5 && (
            <Link className="inactive-more" href={`/customers`}>
              Показать всех в CRM
            </Link>
          )}
        </section>
      </div>
      </div></details>
    </SectionShell>
  );
}

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

type Employee = {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  role: string;
  status: string;
  branchId?: string;
  branch?: string;
};
export function EmployeesPage() {
  const [items, setItems] = useState<Employee[]>([]);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Employee | null>(null);
  const [msg, setMsg] = useState("");
  const load = async () => {
    try {
      const [data, branchData] = await Promise.all([
        api<Employee[]>("/employees"),
        api<Branch[]>("/branches"),
      ]);
      setItems(data);
      setBranches(branchData);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  };
  useEffect(() => {
    void load();
  }, []);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    try {
      await api("/employees", {
        method: "POST",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setOpen(false);
      setMsg("Сотрудник создан");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    try {
      await api(`/employees/${editing.id}`, {
        method: "PATCH",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setEditing(null);
      setMsg("Данные сотрудника сохранены");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function remove(id: string) {
    if (!confirm("Удалить сотрудника?")) return;
    try {
      await api(`/employees/${id}`, { method: "DELETE" });
      setMsg("Сотрудник удалён");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  return (
    <SectionShell
      active="/employees"
      title="Сотрудники"
      subtitle="Доступы команды и привязка к филиалам"
    >
      <Notice text={msg} />
      <div className="toolbar">
        <span>{items.length} пользователей</span>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Plus />
          Добавить сотрудника
        </button>
      </div>
      <div className="data-card">
        <table>
          <thead>
            <tr>
              <th>Сотрудник</th>
              <th>Email</th>
              <th>Роль</th>
              <th>Филиал</th>
              <th>Статус</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((x) => (
              <tr key={x.id}>
                <td>
                  <strong>
                    {x.firstName} {x.lastName}
                  </strong>
                </td>
                <td>{x.email}</td>
                <td>
                  <span className="tag">{x.role}</span>
                </td>
                <td>{x.branch || "Все филиалы"}</td>
                <td>{x.status}</td>
                <td>
                  <div className="row-actions">
                    {x.role === "employee" && (
                      <>
                        <button
                          aria-label="Редактировать сотрудника"
                          onClick={() => setEditing(x)}
                        >
                          <Pencil />
                        </button>
                        <button
                          aria-label="Удалить сотрудника"
                          className="danger-icon"
                          onClick={() => remove(x.id)}
                        >
                          <Trash2 />
                        </button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {open && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={create}>
            <h2>Новый сотрудник</h2>
            <label>
              Имя
              <input name="firstName" required />
            </label>
            <label>
              Фамилия
              <input name="lastName" />
            </label>
            <label>
              Email
              <input name="email" type="email" required />
            </label>
            <label>
              Временный пароль
              <input name="password" type="password" minLength={8} required />
            </label>
            <label>
              Филиал
              <select name="branchId">
                <option value="">Все филиалы</option>
                {branches.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            </label>
            <div>
              <button type="button" onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button className="primary-action">Создать</button>
            </div>
          </form>
        </div>
      )}
      {editing && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={save}>
            <h2>Редактировать сотрудника</h2>
            <label>
              Имя
              <input
                name="firstName"
                defaultValue={editing.firstName}
                required
              />
            </label>
            <label>
              Фамилия
              <input name="lastName" defaultValue={editing.lastName} />
            </label>
            <label>
              Email
              <input
                name="email"
                type="email"
                defaultValue={editing.email}
                required
              />
            </label>
            <label>
              Новый пароль <small>оставьте пустым, чтобы не менять</small>
              <input name="password" type="password" minLength={8} />
            </label>
            <label>
              Филиал
              <select name="branchId" defaultValue={editing.branchId || ""}>
                <option value="">Все филиалы</option>
                {branches.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Статус
              <select name="status" defaultValue={editing.status}>
                <option value="active">Активен</option>
                <option value="blocked">Заблокирован</option>
              </select>
            </label>
            <div>
              <button type="button" onClick={() => setEditing(null)}>
                Отмена
              </button>
              <button className="primary-action">Сохранить</button>
            </div>
          </form>
        </div>
      )}
    </SectionShell>
  );
}

type Device = {
  id: string;
  branchId: string;
  branch: string;
  kind: string;
  name: string;
  url: string;
  destination: string;
  active: boolean;
  scans: number;
};
export function LegacyDevicesPage() {
  const [items, setItems] = useState<Device[]>([]);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Device | null>(null);
  const [msg, setMsg] = useState("");
  const load = async () => {
    try {
      const [data, branchData] = await Promise.all([
        api<Device[]>("/devices"),
        api<Branch[]>("/branches"),
      ]);
      setItems(data);
      setBranches(branchData);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  };
  useEffect(() => {
    void load();
  }, []);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    try {
      await api("/devices", {
        method: "POST",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setOpen(false);
      setMsg("Устройство создано");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    const data = Object.fromEntries(new FormData(e.currentTarget));
    try {
      await api(`/devices/${editing.id}`, {
        method: "PATCH",
        body: JSON.stringify({ ...data, active: editing.active }),
      });
      setEditing(null);
      setMsg("Устройство обновлено");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function toggle(item: Device) {
    try {
      await api(`/devices/${item.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          branchId: item.branchId,
          kind: item.kind,
          name: item.name,
          destination: item.destination,
          active: !item.active,
        }),
      });
      setMsg(item.active ? "Устройство отключено" : "Устройство включено");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function remove(id: string) {
    if (!confirm("Удалить устройство?")) return;
    await api(`/devices/${id}`, { method: "DELETE" });
    setMsg("Устройство удалено");
    await load();
  }
  const destinationOptions = (
    <>
      <option value="join">Регистрация клиента</option>
      <option value="reviews">Отзывы</option>
      <option value="website">Мини-сайт</option>
      <option value="booking">Онлайн-запись</option>
    </>
  );
  return (
    <SectionShell
      active="/devices"
      title="NFC и QR"
      subtitle="Точки регистрации клиентов"
    >
      <Notice text={msg} />
      <div className="toolbar">
        <span>{items.length} устройств</span>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Plus />
          Новое устройство
        </button>
      </div>
      <div className="card-grid">
        {items.map((x) => (
          <article className="entity-card" key={x.id}>
            <span className="entity-icon">
              {x.kind === "nfc" ? <Nfc /> : <QrCode />}
            </span>
            <div>
              <h2>{x.name}</h2>
              <p>
                {x.branch} · {x.scans} сканирований
              </p>
              <span className={x.active ? "status" : "tag"}>
                {x.active ? "Активно" : "Отключено"}
              </span>
              <a href={x.url} target="_blank" rel="noreferrer">
                {x.url}
              </a>
            </div>
            <div className="row-actions">
              <button
                aria-label="Редактировать устройство"
                onClick={() => setEditing(x)}
              >
                <Pencil />
              </button>
              <button
                aria-label={
                  x.active ? "Отключить устройство" : "Включить устройство"
                }
                onClick={() => toggle(x)}
              >
                {x.active ? <ToggleRight /> : <ToggleLeft />}
              </button>
              <button
                aria-label="Удалить устройство"
                className="danger-icon"
                onClick={() => remove(x.id)}
              >
                <Trash2 />
              </button>
            </div>
          </article>
        ))}
      </div>
      {open && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={create}>
            <h2>Новое устройство</h2>
            <label>
              Тип
              <select name="kind">
                <option value="nfc">NFC</option>
                <option value="qr">QR</option>
              </select>
            </label>
            <label>
              Название
              <input name="name" required placeholder="Reception" />
            </label>
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
              Назначение<select name="destination">{destinationOptions}</select>
            </label>
            <div>
              <button type="button" onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button className="primary-action">Создать</button>
            </div>
          </form>
        </div>
      )}
      {editing && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={save}>
            <h2>Редактировать устройство</h2>
            <label>
              Тип
              <select name="kind" defaultValue={editing.kind}>
                <option value="nfc">NFC</option>
                <option value="qr">QR</option>
              </select>
            </label>
            <label>
              Название
              <input name="name" defaultValue={editing.name} required />
            </label>
            <label>
              Филиал
              <select name="branchId" defaultValue={editing.branchId} required>
                {branches.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Назначение
              <select name="destination" defaultValue={editing.destination}>
                {destinationOptions}
              </select>
            </label>
            <div>
              <button type="button" onClick={() => setEditing(null)}>
                Отмена
              </button>
              <button className="primary-action">Сохранить</button>
            </div>
          </form>
        </div>
      )}
    </SectionShell>
  );
}

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

type AnalyticsData = {
  days: number;
  totals: { customers: number; visits: number; pointsIssued: number; pointsRedeemed: number; outstanding: number };
  previous: { visits: number; active: number; new: number; pointsIssued: number };
  series: { date: string; customers: number; visits: number; points: number; firstVisits: number; repeatVisits: number }[];
  audience: {
    active: number;
    returning: number;
    repeatActive: number;
    frequent: number;
    loyal: number;
    atRisk: number;
    new: number;
    retentionRate: number;
    averageVisits: number;
  };
  topCustomers: {
    id: string;
    name: string;
    visits: number;
    points: number;
    level: string;
  }[];
  peakHour: number;
};
type AnalyticsSubscription = { plan: string; status: string };
type ProAnalytics = {
  currency: string;
  repeatPurchase: { windows: { days: number; customers: number; repeatCustomers: number; repeatPurchaseRate: number }[]; averageDaysToSecondPurchase: number; secondPurchaseConversion: number };
  averageCheck: { overall: number; participants: number; anonymous: number; newCustomers: number; repeatCustomers: number };
  ltv: { type: string; customers: number; totalRevenue: number; average: number; median: number; maximum: number };
  branches: { id: string; name: string; transactions: number; customers: number; revenue: number; averageCheck: number }[];
  rfm: { segments: { code: string; name: string; churnRisk: string; customers: number; revenue: number; averageLTV: number }[] };
};
type BonusLiability = { currency: string; issued: number; active: number; redeemed: number; expired: number; liability: number; expectedRedemptionCost: number };
type BusinessOutcomes = {days:number;retention:{returnedCustomers:number;repeatVisits:number};automations:{messages:number;reachedCustomers:number;returnedCustomers:number;attributedRevenue:number};referrals:{newCustomers:number;repeatCustomers:number;revenue:number};rewards:{bestName:string;redemptions:number};revenue:{members:number;campaignAttributed:number};previous:{returnedCustomers:number;automationReturned:number;automationRevenue:number;referralCustomers:number;referralRevenue:number;rewardRedemptions:number;memberRevenue:number};branches:{id:string;name:string;customers:number;returnedCustomers:number;visits:number;revenue:number;rewards:number}[]};
function MetricDelta({ current, previous }: { current: number; previous: number }) {
  const value = previous === 0 ? (current > 0 ? 100 : 0) : ((current - previous) / previous) * 100;
  return <small className={value >= 0 ? "metric-delta up" : "metric-delta down"}>{value >= 0 ? "+" : ""}{value.toFixed(0)}% к прошлому периоду</small>;
}
function OutcomeDelta({current,previous}:{current:number;previous:number}){
  if(previous===0)return current>0?<small className="outcome-delta new">Новый результат за период</small>:<small className="outcome-delta neutral">Без изменений</small>;
  const value=(current-previous)/previous*100;
  return <small className={`outcome-delta ${value>=0?"up":"down"}`}>{value>=0?"↑":"↓"} {Math.abs(value).toFixed(0)}% к прошлому периоду</small>;
}
export function AnalyticsPage() {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [subscription, setSubscription] = useState<AnalyticsSubscription | null>(null);
  const [proData, setProData] = useState<ProAnalytics | null>(null);
  const [liability, setLiability] = useState<BonusLiability | null>(null);
  const [outcomes, setOutcomes] = useState<BusinessOutcomes | null>(null);
  const [period, setPeriod] = useState("month");
  const [msg, setMsg] = useState("");
  useEffect(() => {
    setData(null);setMsg("");
    const days=period==="week"?7:period==="quarter"?90:30;
    Promise.all([api<AnalyticsData>(`/analytics?period=${period}`),api<AnalyticsSubscription>("/subscription"),api<BusinessOutcomes>(`/analytics/outcomes?days=${days}`)])
      .then(([analytics, plan, result]) => {setData(analytics);setSubscription(plan);setOutcomes(result);const normalized=plan.plan.toLowerCase()==="business"||plan.plan.toLowerCase()==="growth"?"growth":plan.plan.toLowerCase();if(normalized==="pro")return Promise.all([api<ProAnalytics>("/analytics/business"),api<BonusLiability>("/analytics/bonus-liability")]).then(([business,bonus])=>{setProData(business);setLiability(bonus)});setProData(null);setLiability(null)})
      .catch((e) => setMsg(e.message));
  }, [period]);
  const tier = subscription?.plan.toLowerCase()==="business"?"growth":subscription?.plan.toLowerCase()||"starter";
  const tierName = tier==="pro"?"Pro":tier==="growth"?"Growth":"Starter";
  const money = (value:number) => `${Math.round(value).toLocaleString("ru-RU")} ₸`;
  const max = Math.max(1, ...(data?.series.map((x) => x.visits) || [1]));
  return (
    <SectionShell
      active="/analytics"
      title="Аналитика"
      subtitle="Рост клиентов, посещения и начисления"
    >
      <Notice text={msg} />
      <section className={`analytics-plan analytics-plan-${tier}`}><div><span>{tier==="pro"?<Crown/>:tier==="growth"?<TrendingUp/>:<BarChart3/>}</span><div><small>АНАЛИТИКА {tierName.toLocaleUpperCase("ru-RU")}</small><h2>{tier==="pro"?"Финансовый центр сети":tier==="growth"?"Центр удержания клиентов":"Пульс программы лояльности"}</h2><p>{tier==="pro"?"Выручка, LTV, повторные покупки и обязательства по бонусам.":tier==="growth"?"Сегменты, риск ухода и конкретные аудитории для возврата.":"Только главные показатели без сложных отчётов."}</p></div></div><Link href="/subscription">{tier==="pro"?"Ваш максимальный тариф":"Сравнить тарифы"}</Link></section>
      <div className="toolbar">
        <span>Данные в реальном времени</span>
        <select aria-label="Период аналитики" value={period} onChange={(e) => setPeriod(e.target.value)}>
          <option value="week">7 дней</option>
          <option value="month">30 дней</option>
          <option value="quarter">90 дней</option>
        </select>
      </div>
      {data && (
        <>
          {outcomes&&<section className="business-outcomes"><header><div><small>РЕЗУЛЬТАТ ПРОГРАММЫ</small><h2>Что лояльность дала бизнесу?</h2><p>Только подтверждённые события за {outcomes.days} дней. Выручка без контрольной группы называется атрибутированной.</p></div><TrendingUp/></header><div>
            <article><span><Repeat2/></span><div><small>Возвращаются ли клиенты?</small><strong>{outcomes.retention.returnedCustomers} клиентов вернулись</strong><p>{outcomes.retention.repeatVisits} повторных посещений за период</p><OutcomeDelta current={outcomes.retention.returnedCustomers} previous={outcomes.previous.returnedCustomers}/></div></article>
            <article><span><Send/></span><div><small>Что дали автоматизации?</small><strong>{outcomes.automations.returnedCustomers} клиентов вернулись</strong><p>{outcomes.automations.attributedRevenue>0?`${money(outcomes.automations.attributedRevenue)} атрибутированной выручки`:`${outcomes.automations.reachedCustomers} клиентов получили сообщение`}</p><OutcomeDelta current={outcomes.automations.returnedCustomers} previous={outcomes.previous.automationReturned}/></div></article>
            <article><span><Users/></span><div><small>Работают ли рекомендации?</small><strong>{outcomes.referrals.newCustomers} новых клиентов</strong><p>{outcomes.referrals.repeatCustomers} уже совершили повторную покупку</p><OutcomeDelta current={outcomes.referrals.newCustomers} previous={outcomes.previous.referralCustomers}/></div></article>
            <article><span><Gift/></span><div><small>Какая награда работает лучше?</small><strong>{outcomes.rewards.bestName||"Пока недостаточно данных"}</strong><p>{outcomes.rewards.redemptions?`${outcomes.rewards.redemptions} использований`:`Появится после первого погашения`}</p><OutcomeDelta current={outcomes.rewards.redemptions} previous={outcomes.previous.rewardRedemptions}/></div></article>
          </div>{outcomes.branches.length>1&&<section className="outcome-branches"><header><div><small>ФИЛИАЛЫ</small><h3>Где программа лучше возвращает клиентов?</h3></div><Link href="/branches">Управлять филиалами</Link></header><div>{outcomes.branches.slice(0,4).map((branch,index)=><article key={branch.id}><b>{index+1}</b><span><strong>{branch.name}</strong><small>{branch.returnedCustomers} вернулись · {branch.visits} посещений</small></span><div><strong>{money(branch.revenue)}</strong><small>{branch.rewards} наград выдано</small></div></article>)}</div></section>}</section>}
          <div className="insight-metrics">
            <article>
              <Users />
              <span>Активных гостей</span>
              <strong>{data.audience.active}</strong>
              <MetricDelta current={data.audience.active} previous={data.previous.active} />
            </article>
            <article>
              <Building2 />
              <span>Посещений за период</span>
              <strong>{data.totals.visits}</strong>
              <MetricDelta current={data.totals.visits} previous={data.previous.visits} />
            </article>
            {tier==="starter"&&<article>
              <UserCheck />
              <span>Новых гостей</span>
              <strong>{data.audience.new}</strong>
              <MetricDelta current={data.audience.new} previous={data.previous.new} />
            </article>}
            {tier!=="starter"&&<article>
              <Repeat2 />
              <span>Возвращаются</span>
              <strong>{data.audience.retentionRate.toFixed(0)}%</strong>
              <small>
                {data.audience.returning} клиентов с повторным визитом
              </small>
            </article>}
            {tier!=="starter"&&<article className="risk-metric">
              <AlertTriangle />
              <span>Риск ухода</span>
              <strong>{data.audience.atRisk}</strong>
              <small>Не возвращались более 45 дней</small>
            </article>}
          </div>
          {tier==="starter"&&<section className="starter-pulse"><div><small>ПУЛЬС ЗА ПЕРИОД</small><strong>{Math.min(100,Math.round(data.audience.retentionRate*.6+Math.min(40,data.audience.active*2)))}</strong><span>из 100</span></div><div><h2>{data.totals.visits?"Программа работает":"Начните собирать визиты"}</h2><p>{data.audience.new>0?`${data.audience.new} новых гостей уже в базе. Следующая цель — вернуть их повторно.`:"Активируйте NFC/QR и зарегистрируйте первых гостей."}</p><Link href={data.totals.visits?"/customers":"/devices"}>{data.totals.visits?"Открыть клиентов":"Настроить NFC/QR"}</Link></div></section>}
          <section className="analytics-answer">
            <TrendingUp />
            <div>
              <span>ГЛАВНЫЙ ВЫВОД</span>
              <h2>
                {data.audience.retentionRate >= 50
                  ? "Клиенты хорошо возвращаются"
                  : "Есть потенциал увеличить повторные визиты"}
              </h2>
              <p>
                {data.audience.atRisk > 0
                  ? `${data.audience.atRisk} клиентам стоит отправить персональное предложение.`
                  : data.audience.new > data.audience.repeatActive
                    ? "Новых гостей больше, чем повторных. Помогите им сделать второй визит."
                    : "Сейчас нет клиентов с высоким риском ухода."}
              </p>
            </div>
            <b>
              {data.audience.averageVisits.toFixed(1)}
              <small>визита в среднем</small>
            </b>
          </section>
          {tier!=="starter"&&<section className="loyalty-economy">
            <header>
              <div><span>ЭКОНОМИКА ПРОГРАММЫ</span><h2>Движение бонусов</h2></div>
              <small>Без выдуманной выручки: показываем только реальные операции Tappix</small>
            </header>
            <div>
              <article><span>Начислено</span><strong>+{data.totals.pointsIssued}</strong><small>за выбранный период</small></article>
              <article><span>Использовано</span><strong>−{data.totals.pointsRedeemed}</strong><small>погашено гостями</small></article>
              <article><span>Баланс у гостей</span><strong>{data.totals.outstanding}</strong><small>доступно сейчас</small></article>
              <article><span>Всего в базе</span><strong>{data.totals.customers}</strong><small>зарегистрированных гостей</small></article>
            </div>
          </section>}
          {tier!=="starter"&&<div className="analytics-business-grid">
            <section className="analytics-segments">
              <header>
                <div>
                  <span>АУДИТОРИЯ</span>
                  <h2>Сегменты клиентов</h2>
                </div>
              </header>
              <div>
                <article>
                  <i className="new" />
                  <span>
                    <strong>{data.audience.new}</strong>
                    <small>Новые</small>
                  </span>
                </article>
                <article>
                  <i className="active" />
                  <span>
                    <strong>{data.audience.frequent}</strong>
                    <small>Частые · 5+ визитов</small>
                  </span>
                </article>
                <article>
                  <i className="loyal" />
                  <span>
                    <strong>{data.audience.loyal}</strong>
                    <small>Постоянные · 10+ визитов</small>
                  </span>
                </article>
                <article>
                  <i className="risk" />
                  <span>
                    <strong>{data.audience.atRisk}</strong>
                    <small>Нужно вернуть</small>
                  </span>
                </article>
              </div>
            </section>
            <section className="analytics-top">
              <header>
                <div>
                  <span>ТОП ГОСТЕЙ</span>
                  <h2>Самые лояльные клиенты</h2>
                </div>
                <Clock3 />
                <small>
                  Пиковое время: {String(data.peakHour).padStart(2, "0")}:00
                </small>
              </header>
              {data.topCustomers.map((customer, index) => (
                <Link href={`/customers/${customer.id}`} key={customer.id}>
                  <b>{index + 1}</b>
                  <span>
                    <strong>{customer.name}</strong>
                    <small>
                      {customer.level} · {customer.points} бонусов
                    </small>
                  </span>
                  <i>{customer.visits} виз.</i>
                </Link>
              ))}
            </section>
          </div>}
          {tier==="growth"&&<section className="growth-actions"><header><small>УНИКАЛЬНО ДЛЯ GROWTH</small><h2>Готовые аудитории для роста</h2><p>Не просто цифры — группы клиентов, с которыми можно работать прямо сейчас.</p></header><div><Link href="/customers"><span><UserX/></span><strong>{data.audience.atRisk} нужно вернуть</strong><small>Не были более 45 дней</small></Link><Link href="/campaigns"><span><UserCheck/></span><strong>{data.audience.new} новых гостей</strong><small>Помогите совершить второй визит</small></Link><Link href="/customers"><span><Star/></span><strong>{data.audience.loyal} постоянных</strong><small>Предложите VIP-награду</small></Link></div></section>}
          {tier==="pro"&&proData&&liability&&<section className="pro-intelligence"><header><div><small>УНИКАЛЬНО ДЛЯ PRO</small><h2>Экономика лояльности</h2><p>Метрики строятся по реальным закрытым чекам из POS.</p></div><Crown/></header><div className="pro-finance-grid"><article><Banknote/><span>Выручка участников</span><strong>{money(proData.ltv.totalRevenue)}</strong><small>{proData.ltv.customers} покупателей</small></article><article><TrendingUp/><span>Historical LTV</span><strong>{money(proData.ltv.average)}</strong><small>медиана {money(proData.ltv.median)}</small></article><article><CreditCard/><span>Средний чек</span><strong>{money(proData.averageCheck.overall)}</strong><small>участники {money(proData.averageCheck.participants)}</small></article><article><Repeat2/><span>Repeat purchase 30 дней</span><strong>{(proData.repeatPurchase.windows.find(x=>x.days===30)?.repeatPurchaseRate||0).toFixed(1)}%</strong><small>до второй покупки {proData.repeatPurchase.averageDaysToSecondPurchase.toFixed(1)} дн.</small></article><article className="liability"><Gift/><span>Bonus liability</span><strong>{money(liability.liability)}</strong><small>ожидаемое погашение {money(liability.expectedRedemptionCost)}</small></article></div>{proData.branches.length>0?<div className="pro-branches"><h3>Филиалы по выручке</h3>{proData.branches.slice(0,5).map((branch,index)=><article key={branch.id||branch.name}><b>{index+1}</b><span><strong>{branch.name}</strong><small>{branch.transactions} чеков · средний {money(branch.averageCheck)}</small></span><em>{money(branch.revenue)}</em></article>)}</div>:<div className="pro-empty"><Building2/><div><strong>Подключите POS, чтобы увидеть экономику</strong><small>После импорта чеков появятся LTV, средний чек, repeat purchase и рейтинг филиалов.</small></div><Link href="/integrations">Подключить интеграцию</Link></div>}</section>}
          <div className="analytics-chart">
            <div className="chart-heading">
              <div><BarChart3 /><span><b>Новые и повторные визиты</b><small>Показывает, возвращаются ли гости после первого знакомства</small></span></div>
              <div className="chart-legend"><i className="first" />Первый визит <i className="repeat" />Повторный</div>
            </div>
            <div className="visit-bars">
              {data.series.map((x, i) => (
                <div key={i} title={`${new Date(x.date).toLocaleDateString("ru-RU")}: ${x.firstVisits} новых, ${x.repeatVisits} повторных`}>
                  <i className="repeat" style={{ height: `${(x.repeatVisits / max) * 100}%` }} />
                  <i className="first" style={{ height: `${Math.max(x.firstVisits ? 3 : 0, (x.firstVisits / max) * 100)}%` }} />
                </div>
              ))}
            </div>
            <footer><span>{new Date(data.series[0]?.date).toLocaleDateString("ru-RU", { day: "numeric", month: "short" })}</span><span>Сегодня</span></footer>
          </div>
        </>
      )}
    </SectionShell>
  );
}

type CompanySettings = {
  name: string;
  phone: string;
  email: string;
  address: string;
  timezone: string;
  language: string;
};
type ActiveSession = {
  id: string;
  createdAt: string;
  userAgent?: string;
  ip?: string;
};
export function SettingsPage() {
  const [value, setValue] = useState<CompanySettings>({
    name: "",
    phone: "",
    email: "",
    address: "",
    timezone: "Asia/Almaty",
    language: "ru",
  });
  const [msg, setMsg] = useState("");
  const [sessions, setSessions] = useState<ActiveSession[]>([]);
  useEffect(() => {
    Promise.all([
      api<CompanySettings>("/settings/company"),
      api<ActiveSession[]>("/auth/sessions"),
    ])
      .then(([settings, active]) => {
        setValue(settings);
        setSessions(active);
      })
      .catch((e) => setMsg(e.message));
  }, []);
  async function save(e: FormEvent) {
    e.preventDefault();
    try {
      const saved = await api<CompanySettings>("/settings/company", {
        method: "PATCH",
        body: JSON.stringify(value),
      });
      setValue(saved);
      setMsg("Настройки компании сохранены");
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  }
  async function revokeSession(id: string) {
    try {
      await api(`/auth/sessions/${id}`, { method: "DELETE" });
      setSessions((current) => current.filter((session) => session.id !== id));
      setMsg("Сессия завершена");
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Не удалось завершить сессию");
    }
  }
  return (
    <SectionShell
      active="/settings"
      title="Настройки"
      subtitle="Основные данные компании"
    >
      <Notice text={msg} />
      <form className="settings-card" onSubmit={save}>
        <div className="form-grid">
          {Object.entries(value).map(([key, val]) => (
            <label key={key}>
              {
                (
                  {
                    name: "Название",
                    phone: "Телефон",
                    email: "Email",
                    address: "Адрес",
                    timezone: "Часовой пояс",
                    language: "Язык",
                  } as Record<string, string>
                )[key]
              }
              <input
                value={val}
                onChange={(e) => setValue({ ...value, [key]: e.target.value })}
              />
            </label>
          ))}
        </div>
        <button className="primary-action">Сохранить</button>
      </form>
      <div className="settings-card session-card">
        <div className="settings-title">
          <span>
            <ShieldCheck />
          </span>
          <div>
            <h2>Активные сессии</h2>
            <p>Устройства, на которых выполнен вход в ваш аккаунт.</p>
          </div>
        </div>
        {sessions.length === 0 ? (
          <p className="session-empty">Активные refresh-сессии не найдены.</p>
        ) : (
          sessions.map((session) => (
            <div className="session-row" key={session.id}>
              <div>
                <strong>
                  {session.userAgent?.includes("Mobile")
                    ? "Мобильное устройство"
                    : "Браузер"}
                </strong>
                <small>
                  {new Date(session.createdAt).toLocaleString("ru-RU")} ·{" "}
                  {session.ip || "IP не определён"}
                </small>
              </div>
              <button type="button" onClick={() => revokeSession(session.id)}>
                Завершить
              </button>
            </div>
          ))
        )}
      </div>
    </SectionShell>
  );
}

type ReviewSettings = {
  gisUrl: string;
  googleUrl: string;
  yandexUrl: string;
  redirectThreshold: number;
  enabled: boolean;
};
export function ReviewsPage() {
  const [value, setValue] = useState<ReviewSettings>({
    gisUrl: "",
    googleUrl: "",
    yandexUrl: "",
    redirectThreshold: 4,
    enabled: false,
  });
  const [msg, setMsg] = useState("");
  useEffect(() => {
    api<ReviewSettings>("/reviews/settings")
      .then(setValue)
      .catch((e) => setMsg(e.message));
  }, []);
  async function save(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/reviews/settings", {
        method: "PATCH",
        body: JSON.stringify(value),
      });
      setMsg("Настройки отзывов сохранены");
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  }
  return (
    <SectionShell
      active="/reviews"
      title="Отзывы"
      subtitle="2GIS, Google Maps и Яндекс Карты"
    >
      <Notice text={msg} />
      <form className="settings-card" onSubmit={save}>
        <div className="settings-title">
          <span>
            <Star />
          </span>
          <div>
            <h2>Перенаправление отзывов</h2>
            <p>Предлагайте довольным клиентам оставить публичный отзыв.</p>
          </div>
        </div>
        <div className="form-grid">
          <label>
            Ссылка 2GIS
            <input
              value={value.gisUrl}
              onChange={(e) => setValue({ ...value, gisUrl: e.target.value })}
            />
          </label>
          <label>
            Ссылка Google Maps
            <input
              value={value.googleUrl}
              onChange={(e) =>
                setValue({ ...value, googleUrl: e.target.value })
              }
            />
          </label>
          <label>
            Ссылка Яндекс
            <input
              value={value.yandexUrl}
              onChange={(e) =>
                setValue({ ...value, yandexUrl: e.target.value })
              }
            />
          </label>
          <label>
            Порог оценки
            <input
              type="number"
              min="1"
              max="5"
              step="0.5"
              value={value.redirectThreshold}
              onChange={(e) =>
                setValue({
                  ...value,
                  redirectThreshold: Number(e.target.value),
                })
              }
            />
          </label>
        </div>
        <label className="toggle-row">
          <input
            type="checkbox"
            checked={value.enabled}
            onChange={(e) => setValue({ ...value, enabled: e.target.checked })}
          />
          Модуль отзывов включён
        </label>
        <button className="primary-action">Сохранить</button>
      </form>
    </SectionShell>
  );
}

type Audit = {
  id: number;
  action: string;
  entityType: string;
  requestId: string;
  ip?: string;
  createdAt: string;
  user: string;
  company: string;
};
type OperationApproval={id:string;operation:"bonus.credit"|"bonus.debit";amount:number;reason:string;status:string;requestedAt:string;expiresAt:string;customerId:string;customer:string;branch:string;requester:string};
export function AuditPage() {
  const [items, setItems] = useState<Audit[]>([]);
  const [approvals,setApprovals]=useState<OperationApproval[]>([]);
  const [decision,setDecision]=useState<{id:string;value:"approved"|"rejected"}|null>(null);
  const [saving,setSaving]=useState(false);
  const [msg, setMsg] = useState("");
  const decisionDialogRef=useRef<HTMLFormElement>(null);
  const decisionTriggerRef=useRef<HTMLElement|null>(null);
  function load(){return Promise.all([api<Audit[]>("/audit"),api<OperationApproval[]>("/operation-approvals?status=pending")]).then(([audit,pending])=>{setItems(audit);setApprovals(pending)}).catch((e)=>setMsg(e.message))}
  useEffect(() => {void load()}, []);
  useEffect(()=>{
    if(!decision)return;
    decisionTriggerRef.current=document.activeElement as HTMLElement;
    const onKeyDown=(event:KeyboardEvent)=>{
      if(event.key==="Escape"){event.preventDefault();setDecision(null);return}
      if(event.key!=="Tab"||!decisionDialogRef.current)return;
      const focusable=Array.from(decisionDialogRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), textarea:not([disabled])'));
      if(!focusable.length)return;
      const first=focusable[0];const last=focusable[focusable.length-1];
      if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus()}
      else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus()}
    };
    document.addEventListener("keydown",onKeyDown);
    return()=>{document.removeEventListener("keydown",onKeyDown);decisionTriggerRef.current?.focus()};
  },[decision]);
  async function submitDecision(event:FormEvent<HTMLFormElement>){event.preventDefault();if(!decision)return;const data=new FormData(event.currentTarget);setSaving(true);setMsg("");try{await api(`/operation-approvals/${decision.id}/decision`,{method:"POST",body:JSON.stringify({decision:decision.value,reason:String(data.get("reason")||"")})});setDecision(null);await load()}catch(error){setMsg(error instanceof Error?error.message:"Не удалось сохранить решение")}finally{setSaving(false)}}
  return (
    <SectionShell
      active="/audit"
      title="Журнал аудита"
      subtitle="Кто, когда и что изменил"
    >
      <Notice text={msg} />
      <section className="approval-queue"><header><div><small>КОНТРОЛЬ ОПЕРАЦИЙ</small><h2>Требуют вашего решения</h2><p>Крупные ручные начисления и списания не выполняются без подтверждения владельца.</p></div><b>{approvals.length}</b></header>{approvals.length?<div>{approvals.map(item=><article key={item.id}><span className={item.operation==="bonus.debit"?"debit":"credit"}>{item.operation==="bonus.debit"?"−":"+"}{item.amount.toLocaleString("ru-RU")}</span><div><strong>{item.customer}</strong><small>{item.branch} · сотрудник {item.requester}</small><p>{item.reason}</p><time>До {new Date(item.expiresAt).toLocaleString("ru-RU")}</time></div><nav><button onClick={()=>setDecision({id:item.id,value:"rejected"})}>Отклонить</button><button onClick={()=>setDecision({id:item.id,value:"approved"})}><Check/>Одобрить</button></nav></article>)}</div>:<div className="approval-empty"><ShieldCheck/><span><strong>Нет заявок на подтверждение</strong><small>Все крупные операции рассмотрены.</small></span></div>}</section>
      <div className="data-card">
        <table>
          <thead>
            <tr>
              <th>Время</th>
              <th>Пользователь</th>
              <th>Действие</th>
              <th>IP</th>
              <th>Request ID</th>
            </tr>
          </thead>
          <tbody>
            {items.map((x) => (
              <tr key={x.id}>
                <td>{new Date(x.createdAt).toLocaleString("ru-RU")}</td>
                <td>
                  <strong>{x.user}</strong>
                </td>
                <td>{x.action}</td>
                <td>{x.ip || "—"}</td>
                <td>
                  <code>{x.requestId}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!items.length && (
          <div className="zero">
            <strong>Журнал пока пуст</strong>
            <p>Новые изменения появятся здесь.</p>
          </div>
        )}
      </div>
      {decision&&<div className="approval-dialog-backdrop" role="presentation" onMouseDown={()=>setDecision(null)}><form ref={decisionDialogRef} className="approval-dialog" role="dialog" aria-modal="true" aria-labelledby="approval-dialog-title" aria-describedby="approval-dialog-description" onSubmit={submitDecision} onMouseDown={event=>event.stopPropagation()}><span>{decision.value==="approved"?<ShieldCheck/>:<ShieldAlert/>}</span><h2 id="approval-dialog-title">{decision.value==="approved"?"Одобрить операцию?":"Отклонить операцию?"}</h2><p id="approval-dialog-description">{decision.value==="approved"?"Бонусная операция будет выполнена сразу после подтверждения.":"Баланс клиента не изменится, сотрудник увидит отклонённую заявку."}</p><label>Причина решения<textarea name="reason" minLength={4} placeholder={decision.value==="approved"?"Проверил чек и подтверждаю":"Например: сумма не совпадает с чеком"} autoFocus required/></label><div><button type="button" onClick={()=>setDecision(null)}>Отмена</button><button disabled={saving}>{saving?"Сохраняем…":decision.value==="approved"?"Одобрить и выполнить":"Отклонить"}</button></div></form></div>}
    </SectionShell>
  );
}

type Notification = {
  id: string;
  recipient: string;
  subject: string;
  body: string;
  status: string;
  createdAt: string;
};
export function NotificationsPage() {
  const [items, setItems] = useState<Notification[]>([]);
  const [open, setOpen] = useState(false);
  const [msg, setMsg] = useState("");
  const load = () =>
    api<Notification[]>("/notifications")
      .then(setItems)
      .catch((e) => setMsg(e.message));
  useEffect(() => {
    void load();
  }, []);
  async function send(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    try {
      await api("/notifications/send", {
        method: "POST",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setOpen(false);
      setMsg("Письмо отправлено");
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка отправки");
    }
  }
  return (
    <SectionShell
      active="/notifications"
      title="Рассылки"
      subtitle="Email-уведомления клиентам"
    >
      <Notice text={msg} />
      <div className="toolbar">
        <a href="http://localhost:8025" target="_blank">
          Открыть локальный почтовый ящик
        </a>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Send />
          Новое письмо
        </button>
      </div>
      <div className="data-card">
        <table>
          <thead>
            <tr>
              <th>Получатель</th>
              <th>Тема</th>
              <th>Статус</th>
              <th>Дата</th>
            </tr>
          </thead>
          <tbody>
            {items.map((x) => (
              <tr key={x.id}>
                <td>{x.recipient}</td>
                <td>
                  <strong>{x.subject}</strong>
                </td>
                <td>
                  <span
                    className={
                      x.status === "sent" ? "mail-sent" : "mail-failed"
                    }
                  >
                    {x.status}
                  </span>
                </td>
                <td>{new Date(x.createdAt).toLocaleString("ru-RU")}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!items.length && (
          <div className="zero">
            <Bell />
            <strong>Писем пока нет</strong>
          </div>
        )}
      </div>
      {open && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={send}>
            <h2>Новое письмо</h2>
            <label>
              Email получателя
              <input name="recipient" type="email" required />
            </label>
            <label>
              Тема
              <input name="subject" required />
            </label>
            <label>
              Сообщение
              <textarea name="body" rows={7} required />
            </label>
            <div>
              <button type="button" onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button className="primary-action">Отправить</button>
            </div>
          </form>
        </div>
      )}
    </SectionShell>
  );
}
