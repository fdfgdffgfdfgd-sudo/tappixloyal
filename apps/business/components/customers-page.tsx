"use client";
import { FormEvent, useEffect, useState } from "react";
import { AlertTriangle, FileDown, Plus, RefreshCw, Search, Users } from "lucide-react";
import { api, download } from "@/lib/api";
import { customerLevelLabel } from "@/lib/labels";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Customer, Branch, Notice } from "./management-shared";

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
