"use client";
import { FormEvent, useEffect, useState } from "react";
import { AlertTriangle, FileDown, Plus, RefreshCw, Search, Users } from "lucide-react";
import { api, download } from "@/lib/api";
import { customerLevelLabel } from "@/lib/labels";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Customer, Branch, Notice } from "./management-shared";
import { CustomerMobileCard } from "./owner-ux-primitives";

export function CustomersPage() {
  const [items, setItems] = useState<Customer[]>([]);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [q, setQ] = useState("");
  const [searchQ, setSearchQ] = useState("");
  const [level, setLevel] = useState("");
  const [segment, setSegment] = useState("");
  const [status, setStatus] = useState("");
  const [lastVisit, setLastVisit] = useState("");
  const [rewardState, setRewardState] = useState("");
  const [registeredFrom, setRegisteredFrom] = useState("");
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
    segment,status,lastVisit,rewardState,registeredFrom,
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
      setQ(saved.q || ""); setSearchQ(saved.q || ""); setLevel(saved.level || ""); setSegment(saved.segment||"");setStatus(saved.status||"");setLastVisit(saved.lastVisit||"");setRewardState(saved.rewardState||"");setRegisteredFrom(saved.registeredFrom||"");setBranch(saved.branch || ""); setBirthday(saved.birthday || ""); setMinPoints(saved.minPoints || ""); setSort(saved.sort || "createdAt"); setOrder(saved.order || "desc"); setPage(Number(saved.page) || 1);
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
    sessionStorage.setItem("tappix_customer_filters", JSON.stringify({q:searchQ,level,segment,status,lastVisit,rewardState,registeredFrom,branch,birthday,minPoints,sort,order,page}));
  }, [filtersReady, searchQ, level, segment,status,lastVisit,rewardState,registeredFrom,branch, birthday, minPoints, sort, order, page]);
  useEffect(
    () => setPage(1),
    [searchQ, level, segment,status,lastVisit,rewardState,registeredFrom,branch, birthday, minPoints, sort, order],
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
    setSegment("");setStatus("");setLastVisit("");setRewardState("");setRegisteredFrom("");
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
      subtitle="Все клиенты, их прогресс и последние визиты — в одном месте"
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
      <details className="crm-filter-panel" open>
        <summary><span><Search/>Фильтры и сортировка</span><small>Сегмент, статус, филиал и активность</small></summary>
      <div className="crm-filters" aria-label="Фильтры клиентов">
        <label>Сегмент<select value={segment} onChange={e=>setSegment(e.target.value)}><option value="">Все сегменты</option><option value="new">Новые</option><option value="active">Активные</option><option value="loyal">Лояльные</option><option value="at_risk">Требуют внимания</option><option value="inactive">Неактивные</option></select></label>
        <label>Статус<select value={status} onChange={e=>setStatus(e.target.value)}><option value="">Все</option><option value="active">Активен</option><option value="inactive">Неактивен</option></select></label>
        <label>Последний визит<select value={lastVisit} onChange={e=>setLastVisit(e.target.value)}><option value="">Любая дата</option><option value="30d">За 30 дней</option><option value="90d">За 90 дней</option><option value="older">Более 90 дней назад</option></select></label>
        <label>Награда<select value={rewardState} onChange={e=>setRewardState(e.target.value)}><option value="">Любой прогресс</option><option value="close">Есть прогресс</option><option value="available">Награда доступна</option></select></label>
        <label>Дата вступления<input type="date" value={registeredFrom} onChange={e=>setRegisteredFrom(e.target.value)}/></label>
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
              <th>Сегмент</th>
              <th>Посещения</th>
              <th>Прогресс / баланс</th>
              <th>Последний визит</th>
              <th>Филиал</th>
              <th>Статус</th>
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
                <td data-label="Сегмент">
                  <span className={`customer-segment segment-${c.segment||"new"}`}>{({new:"Новый",active:"Активный",loyal:"Лояльный",at_risk:"Требует внимания",inactive:"Неактивный"} as Record<string,string>)[c.segment||"new"]}</span>
                </td>
                <td data-label="Посещения">{c.totalVisits}</td>
                <td data-label="Баланс">
                  <b>{c.totalPoints} б.</b>
                </td>
                <td data-label="Последний визит">{c.lastVisit?new Date(c.lastVisit).toLocaleDateString("ru-RU"):"Ещё не был"}</td>
                <td data-label="Филиал">{c.lastBranch||"—"}</td>
                <td data-label="Статус"><span className={`owner-status ${c.status||"active"}`}>{c.status==="inactive"?"Неактивен":"Активен"}</span></td>
              </tr>
            ))}
            {loading && Array.from({length:5},(_,index)=><tr className="crm-skeleton" key={index}>{Array.from({length:9},(_,cell)=><td key={cell}><span/></td>)}</tr>)}
          </tbody>
        </table>
        {!loading && !error && !items.length && (
          <div className="zero">
            <Users />
            <strong>{searchQ||segment||branch?"Клиенты не найдены":"Клиентов пока нет"}</strong>
            <p>{searchQ||segment||branch?"Измените фильтры или поисковый запрос.":"Первый клиент появится после регистрации через QR или NFC."}</p>
            {!searchQ&&!segment&&!branch&&<Link className="primary-action" href="/devices">Открыть QR</Link>}
          </div>
        )}
      </div>
      {!loading && !!items.length && <section className="customer-mobile-list" aria-label="Клиенты на мобильном устройстве">
        {items.map((c) => <CustomerMobileCard id={c.id} name={`${c.firstName} ${c.lastName}`} phone={c.phone} level={customerLevelLabel(c.level)} visits={c.totalVisits} points={c.totalPoints} key={`mobile-${c.id}`} />)}
      </section>}
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
