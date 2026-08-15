"use client";
import { useEffect, useState } from "react";
import {
  Activity,
  BarChart3,
  Building2,
  CheckCircle2,
  ChevronRight,
  CircleDollarSign,
  Code2,
  CreditCard,
  Headphones,
  LayoutDashboard,
  LogOut,
  Plus,
  Search,
  Settings,
  ShieldCheck,
  Terminal,
  UserRoundSearch,
  Users,
} from "lucide-react";
import { CompanyProvisioningWizard } from "./company-provisioning-wizard";
import { csrfHeaders } from "@/lib/csrf";
import { API_URL as base } from "@/lib/api";
type Company = {
  id: string;
  name: string;
  slug: string;
  status: string;
  customers: number;
  plan: string;
};
type Stats = {
  companies: number;
  customers: number;
  activeSubscriptions: number;
  monthlyRevenue: number;
};
type PlatformPlan = {
  code: string;
  name: string;
  monthlyPrice: number;
  currency: string;
  status: string;
  entitlements?: { code: string; enabled: boolean; limit?: number }[];
};
type PlatformUser = {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
  role: string;
  status: string;
  company: string;
};
type PlatformAnalytics = {
  companies: number;
  customers: number;
  visits: number;
  frequent: number;
  loyal: number;
  atRisk: number;
  newThisMonth: number;
  retentionRate: number;
  averageVisits: number;
  companyBreakdown: { id: string; name: string; slug: string; customers: number; visits: number; returning: number; atRisk: number }[];
};
type PlatformCustomer = { id:string; companyId:string; company:string; firstName:string; lastName:string; phone:string; points:number; visits:number; level:string; segment:string; lastVisitAt?:string };
type SupportSession = { id:string; companyId:string; company:string; reason:string; createdAt:string; expiresAt:string; active:boolean };
type AuditEvent = { id:number; action:string; entityType:string; requestId:string; ip:string; createdAt:string; user:string; company:string };
const nav = [
  ["overview", LayoutDashboard, "Overview"],
  ["companies", Building2, "Компании"],
  ["users", Users, "Пользователи"],
  ["revenue", CreditCard, "Подписки и оплаты"],
  ["orders", CircleDollarSign, "Заказы"],
  ["support", Headphones, "Поддержка"],
  ["insights", BarChart3, "Статистика"],
  ["developer", Code2, "API и логи"],
  ["settings", Settings, "Настройки"],
] as const;
export function AdminPortal() {
  const [view, setView] = useState("overview"),
    [stats, setStats] = useState<Stats>({
      companies: 0,
      customers: 0,
      activeSubscriptions: 0,
      monthlyRevenue: 0,
    }),
    [companies, setCompanies] = useState<Company[]>([]),
    [plans, setPlans] = useState<PlatformPlan[]>([]),
    [users, setUsers] = useState<PlatformUser[]>([]),
    [platformAnalytics, setPlatformAnalytics] = useState<PlatformAnalytics>({
      companies: 0,
      customers: 0,
      visits: 0,
      frequent: 0,
      loyal: 0,
      atRisk: 0,
      newThisMonth: 0,
      retentionRate: 0,
      averageVisits: 0,
      companyBreakdown: [],
    }),
    [customers, setCustomers] = useState<PlatformCustomer[]>([]),
    [supportSessions, setSupportSessions] = useState<SupportSession[]>([]),
    [audit, setAudit] = useState<AuditEvent[]>([]),
    [supportCompany, setSupportCompany] = useState(""),
    [supportReason, setSupportReason] = useState(""),
    [query, setQuery] = useState(""),
    [open, setOpen] = useState(false),
    [selected, setSelected] = useState<Company | null>(null),
    [msg, setMsg] = useState("");
  async function request(path: string, init: RequestInit = {}) {
    let response = await fetch(`${base}${path}`, {
      ...init,
      credentials: "include",
      headers: { "Content-Type": "application/json", ...csrfHeaders("platform"), ...init.headers },
    });
    if (response.status === 401) {
      const refreshed = await fetch(`${base}/auth/refresh?aud=platform`, {
        method: "POST",
        credentials: "include",
        headers: csrfHeaders("platform"),
      });
      if (refreshed.ok)
        response = await fetch(`${base}${path}`, {
          ...init,
          credentials: "include",
          headers: { "Content-Type": "application/json", ...csrfHeaders("platform"), ...init.headers },
        });
    }
    if (response.status === 401) {
      window.location.assign("/login");
      throw new Error("Войдите в Platform Console");
    }
    const result = await response.json();
    if (!result.success) throw new Error(result.error?.message || "Ошибка");
    return result.data;
  }
  async function load() {
    try {
      const [s, c, p, u, a, customerData, sessions, events] = await Promise.all([
        request("/admin/dashboard"),
        request("/admin/companies"),
        request("/admin/plans"),
        request("/admin/users"),
        request("/admin/analytics"),
        request("/admin/customers"),
        request("/admin/support-sessions"),
        request("/audit"),
      ]);
      setStats(s);
      setCompanies(c);
      setPlans(p);
      setUsers(u);
      setPlatformAnalytics(a);
      setCustomers(customerData);
      setSupportSessions(sessions);
      setAudit(events);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка загрузки");
    }
  }
  async function createSupportSession() {
    try {
      await request("/admin/support-sessions", { method:"POST", body:JSON.stringify({ companyId:supportCompany, reason:supportReason, minutes:30 }) });
      setMsg("Защищённая support-сессия открыта на 30 минут. Все действия попадут в аудит.");
      setSupportReason("");
      await load();
    } catch (e) { setMsg(e instanceof Error ? e.message : "Не удалось открыть support-сессию"); }
  }
  async function changeStatus(status: string) {
    if (!selected) return;
    try {
      await request(`/admin/companies/${selected.id}/status`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
      });
      setMsg(`${selected.name}: статус изменён на ${status}`);
      setSelected(null);
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Не удалось изменить статус");
    }
  }
  async function savePlan(plan: PlatformPlan) {
    try {
      await request(`/admin/plans/${plan.code}`, {
        method: "PATCH",
        body: JSON.stringify({
          name: plan.name,
          monthlyPrice: plan.monthlyPrice,
          status: plan.status,
        }),
      });
      setMsg(`Тариф ${plan.name} сохранён`);
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Не удалось сохранить тариф");
    }
  }
  async function changeCompanyPlan(code: string) {
    if (!selected) return;
    const plan = plans.find((x) => x.code === code);
    if (!plan) return;
    try {
      await request(`/admin/companies/${selected.id}/subscription`, {
        method: "PATCH",
        body: JSON.stringify({
          plan: plan.name,
          status: "active",
          amount: plan.monthlyPrice,
          billingPeriod: "monthly",
          periodEndsAt: "",
          modules: [],
        }),
      });
      setMsg(`${selected.name}: подключён ${plan.name}`);
      setSelected(null);
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Не удалось сменить тариф");
    }
  }
  useEffect(() => {
    void load();
  }, []);
  const filtered = companies.filter((c) =>
    `${c.name} ${c.slug}`.toLowerCase().includes(query.toLowerCase()),
  );
  const pastDue = companies.filter((c) => c.status !== "active").length;
  const operational = {
    users: [
      "Пользователи платформы",
      "Владельцы, сотрудники и гости всех компаний",
      `${stats.customers} гостевых профилей`,
    ],
    revenue: [
      "Подписки и оплаты",
      "Выручка, тарифы и просроченные платежи",
      `${stats.monthlyRevenue.toLocaleString("ru-RU")} ₸ MRR`,
    ],
    orders: [
      "Заказы NFC",
      "Изготовление, доставка и активация носителей",
      "2 демо-носителя активировано",
    ],
    settings: [
      "Настройки платформы",
      "Глобальные тарифы, безопасность и правила",
      "Starter · Growth · Pro",
    ],
  }[view] as string[] | undefined;
  return (
    <div className="platform-shell">
      <aside>
        <div className="platform-brand">
          <span>
            <ShieldCheck />
          </span>
          <div>
            <strong>Tappix</strong>
            <small>Platform Console</small>
          </div>
        </div>
        <nav>
          {nav.map(([id, Icon, label]) => (
            <button
              className={view === id ? "current" : ""}
              onClick={() => setView(id)}
              key={id}
            >
              <Icon />
              {label}
            </button>
          ))}
        </nav>
        <button
          className="platform-exit"
          onClick={async () => {
            await fetch(`${base}/auth/logout?aud=platform`, {
              method: "POST",
              credentials: "include",
              headers: csrfHeaders("platform"),
            });
            location.href = "/login";
          }}
        >
          <LogOut />
          Выйти
        </button>
      </aside>
      <main>
        <header>
          <div>
            <span>FOUNDER CONSOLE</span>
            <h1>{nav.find((x) => x[0] === view)?.[2]}</h1>
            <p>Управление платформой Tappix</p>
          </div>
          <div className="platform-founder">
            <span>А</span>
            <div>
              <strong>Армат</strong>
              <small>Founder · Super Admin</small>
            </div>
          </div>
        </header>
        <section>
          {msg && (
            <div className="platform-notice" role="status">
              {msg}
            </div>
          )}
          {view === "overview" && (
            <>
              <div className="platform-hero">
                <div>
                  <span>Платформа работает</span>
                  <h2>Добро пожаловать, Армат</h2>
                  <p>Выручка, клиенты и риски всех компаний в одном месте.</p>
                </div>
                <button onClick={() => setOpen(true)}>
                  <Plus />
                  Создать компанию
                </button>
              </div>
              <div className="platform-metrics">
                <article>
                  <Building2 />
                  <p>Компаний</p>
                  <strong>{stats.companies}</strong>
                  <small>Всего tenant-пространств</small>
                </article>
                <article>
                  <Users />
                  <p>Клиентов бизнеса</p>
                  <strong>{stats.customers}</strong>
                  <small>Во всех компаниях</small>
                </article>
                <article>
                  <CreditCard />
                  <p>Активных подписок</p>
                  <strong>{stats.activeSubscriptions}</strong>
                  <small>{pastDue} требуют внимания</small>
                </article>
                <article>
                  <CircleDollarSign />
                  <p>MRR</p>
                  <strong>
                    {stats.monthlyRevenue.toLocaleString("ru-RU")} ₸
                  </strong>
                  <small>Текущая месячная выручка</small>
                </article>
              </div>
              <div className="platform-grid">
                <section className="platform-card">
                  <div className="platform-card-head">
                    <div>
                      <h2>Компании</h2>
                      <p>Последние рабочие пространства</p>
                    </div>
                    <button onClick={() => setView("companies")}>
                      Все компании
                    </button>
                  </div>
                  {companies.slice(0, 5).map((c) => (
                    <div className="platform-company" key={c.id}>
                      <span>{c.name.slice(0, 1)}</span>
                      <div>
                        <strong>{c.name}</strong>
                        <small>
                          {c.slug} · {c.customers} клиентов
                        </small>
                      </div>
                      <b>{c.plan}</b>
                      <i className={c.status}>{c.status}</i>
                    </div>
                  ))}
                </section>
                <section className="platform-card">
                  <div className="platform-card-head">
                    <div>
                      <h2>Состояние платформы</h2>
                      <p>Критические контуры</p>
                    </div>
                    <Activity />
                  </div>
                  {[
                    "API и авторизация",
                    "PostgreSQL",
                    "Redis и очереди",
                    "Business application",
                  ].map((x) => (
                    <div className="health-row" key={x}>
                      <span />
                      <strong>{x}</strong>
                      <small>Operational</small>
                    </div>
                  ))}
                </section>
              </div>
            </>
          )}
          {view === "companies" && (
            <>
              <div className="platform-toolbar">
                <label>
                  <Search />
                  <input
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    placeholder="Найти компанию или slug"
                  />
                </label>
                <button onClick={() => setOpen(true)}>
                  <Plus />
                  Создать компанию
                </button>
              </div>
              <section className="platform-card platform-table">
                <table>
                  <thead>
                    <tr>
                      <th>Компания</th>
                      <th>Slug</th>
                      <th>Клиенты</th>
                      <th>Тариф</th>
                      <th>Статус</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {filtered.map((c) => (
                      <tr key={c.id}>
                        <td>
                          <strong>{c.name}</strong>
                        </td>
                        <td>{c.slug}</td>
                        <td>{c.customers}</td>
                        <td>{c.plan}</td>
                        <td>
                          <i className={c.status}>{c.status}</i>
                        </td>
                        <td>
                          <button onClick={() => setSelected(c)}>
                            Управлять
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </section>
            </>
          )}
          {view === "support" && (
            <div className="founder-workspace">
              <div className="platform-hero"><div><span>SECURE SUPPORT</span><h2>Центр поддержки компаний</h2><p>Вход в tenant только по причине, на ограниченное время и с полным аудитом.</p></div></div>
              <div className="platform-metrics">
                <article><Headphones/><p>Активные сессии</p><strong>{supportSessions.filter(x=>x.active).length}</strong><small>Открыты сейчас</small></article>
                <article><Building2/><p>Требуют внимания</p><strong>{pastDue}</strong><small>Заблокированы или приостановлены</small></article>
                <article><ShieldCheck/><p>Журналирование</p><strong>100%</strong><small>Каждое действие записывается</small></article>
                <article><Activity/><p>История</p><strong>{supportSessions.length}</strong><small>Последние 100 сессий</small></article>
              </div>
              <div className="platform-grid">
                <section className="platform-card support-launcher"><div className="platform-card-head"><div><h2>Открыть support-сессию</h2><p>Не используйте доступ без обращения или согласованной проверки</p></div></div>
                  <label>Компания<select value={supportCompany} onChange={e=>setSupportCompany(e.target.value)}><option value="">Выберите компанию</option>{companies.filter(x=>x.status==="active").map(x=><option key={x.id} value={x.id}>{x.name}</option>)}</select></label>
                  <label>Причина доступа<textarea value={supportReason} onChange={e=>setSupportReason(e.target.value)} placeholder="Например: обращение #104 — не начислились бонусы"/></label>
                  <button disabled={!supportCompany || supportReason.trim().length<10} onClick={()=>void createSupportSession()}><ShieldCheck/>Открыть на 30 минут</button>
                </section>
                <section className="platform-card"><div className="platform-card-head"><div><h2>Последние сессии</h2><p>Кто и зачем входил в компанию</p></div></div>
                  {supportSessions.length===0 && <p className="empty-copy">Сессий ещё не было.</p>}
                  {supportSessions.slice(0,7).map(x=><div className="support-row" key={x.id}><span className={x.active?"live":""}/><div><strong>{x.company}</strong><small>{x.reason}</small></div><time>{x.active?"Активна":new Date(x.createdAt).toLocaleDateString("ru-RU")}</time></div>)}
                </section>
              </div>
            </div>
          )}
          {view === "insights" && (
            <div className="founder-workspace">
              <div className="platform-hero"><div><span>PLATFORM INTELLIGENCE</span><h2>Статистика всей платформы</h2><p>Рост, удержание и качество клиентской базы в разрезе компаний.</p></div></div>
              <div className="platform-insight-grid detailed">
                <article><strong>{platformAnalytics.customers}</strong><span>Всего гостей</span></article><article><strong>{platformAnalytics.visits}</strong><span>Посещений</span></article><article><strong>{platformAnalytics.retentionRate}%</strong><span>Возвращаемость</span></article><article><strong>{platformAnalytics.averageVisits.toFixed(1)}</strong><span>Визита в среднем</span></article><article className="risk"><strong>{platformAnalytics.atRisk}</strong><span>В риске ухода</span></article>
              </div>
              <div className="platform-grid">
                <section className="platform-card platform-data-table"><div className="platform-card-head"><div><h2>Компании</h2><p>Сравнение активности tenant-пространств</p></div></div><table><thead><tr><th>Компания</th><th>Гости</th><th>Визиты</th><th>Вернулись</th><th>Риск</th></tr></thead><tbody>{platformAnalytics.companyBreakdown.map(x=><tr key={x.id}><td><strong>{x.name}</strong><small>{x.slug}</small></td><td>{x.customers}</td><td>{x.visits}</td><td>{x.returning}</td><td>{x.atRisk}</td></tr>)}</tbody></table></section>
                <section className="platform-card customer-leaderboard"><div className="platform-card-head"><div><h2>Кто пользуется чаще</h2><p>Самые активные гости платформы</p></div><UserRoundSearch/></div>{customers.slice(0,8).map((x,i)=><button key={x.id}><b>{i+1}</b><div><strong>{x.firstName} {x.lastName}</strong><small>{x.company} · {x.segment}</small></div><span>{x.visits} виз.</span></button>)}</section>
              </div>
            </div>
          )}
          {view === "developer" && (
            <div className="founder-workspace">
              <div className="platform-hero"><div><span>OBSERVABILITY</span><h2>API, безопасность и аудит</h2><p>Реальные HTTP-события, request ID, IP и инициатор действия.</p></div></div>
              <div className="platform-metrics"><article><Activity/><p>API</p><strong>Online</strong><small>Контур отвечает</small></article><article><Terminal/><p>Событий</p><strong>{audit.length}</strong><small>Последние записи аудита</small></article><article><Users/><p>Уникальных IP</p><strong>{new Set(audit.map(x=>x.ip).filter(Boolean)).size}</strong><small>В текущей выборке</small></article><article><ShieldCheck/><p>Трассировка</p><strong>{audit.filter(x=>x.requestId).length}</strong><small>С request ID</small></article></div>
              <section className="platform-card platform-data-table audit-table"><div className="platform-card-head"><div><h2>Журнал API и действий</h2><p>Последние 100 событий платформы</p></div></div><table><thead><tr><th>Время</th><th>Действие</th><th>Пользователь</th><th>Компания</th><th>IP</th><th>Request ID</th></tr></thead><tbody>{audit.map(x=><tr key={x.id}><td>{new Date(x.createdAt).toLocaleString("ru-RU")}</td><td><code>{x.action}</code><small>{x.entityType}</small></td><td>{x.user}</td><td>{x.company}</td><td><code>{x.ip||"—"}</code></td><td><code>{x.requestId?.slice(0,12)||"—"}</code></td></tr>)}</tbody></table></section>
            </div>
          )}
          {operational && (
            <div className="platform-operational">
              <div className="platform-hero">
                <div>
                  <span>LIVE PLATFORM DATA</span>
                  <h2>{operational[0]}</h2>
                  <p>{operational[1]}</p>
                </div>
                <button onClick={() => setView("companies")}>
                  <Building2 />
                  Открыть компании
                </button>
              </div>
              <div className="platform-metrics">
                <article>
                  <Activity />
                  <p>Состояние</p>
                  <strong>Online</strong>
                  <small>Данные обновлены сейчас</small>
                </article>
                <article>
                  <Building2 />
                  <p>Компании</p>
                  <strong>{stats.companies}</strong>
                  <small>Tenant-пространства</small>
                </article>
                <article>
                  <Users />
                  <p>Клиенты</p>
                  <strong>{stats.customers}</strong>
                  <small>Во всех компаниях</small>
                </article>
                <article>
                  <CircleDollarSign />
                  <p>Контекст</p>
                  <strong className="metric-context">{operational[2]}</strong>
                  <small>По текущим данным</small>
                </article>
              </div>
              {view === "users" && (
                <section className="platform-card platform-data-table">
                  <div className="platform-card-head">
                    <div>
                      <h2>Все аккаунты</h2>
                      <p>Роли и принадлежность к компаниям</p>
                    </div>
                    <b>{users.length}</b>
                  </div>
                  <table>
                    <thead>
                      <tr>
                        <th>Пользователь</th>
                        <th>Компания</th>
                        <th>Роль</th>
                        <th>Статус</th>
                      </tr>
                    </thead>
                    <tbody>
                      {users.map((user) => (
                        <tr key={user.id}>
                          <td>
                            <strong>
                              {user.firstName} {user.lastName}
                            </strong>
                            <small>{user.email}</small>
                          </td>
                          <td>{user.company}</td>
                          <td>{user.role}</td>
                          <td>
                            <i className={user.status}>{user.status}</i>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </section>
              )}
              {(view === "revenue" || view === "settings") && (
                <section className="platform-plan-editor">
                  <div className="platform-card-head">
                    <div>
                      <h2>Тарифы и цены</h2>
                      <p>Изменения применяются к новым подключениям</p>
                    </div>
                  </div>
                  <div>
                    {plans.map((plan) => (
                      <article key={plan.code}>
                        <span>{plan.code.toUpperCase()}</span>
                        <label>
                          Название
                          <input
                            value={plan.name}
                            onChange={(e) =>
                              setPlans((current) =>
                                current.map((x) =>
                                  x.code === plan.code
                                    ? { ...x, name: e.target.value }
                                    : x,
                                ),
                              )
                            }
                          />
                        </label>
                        <label>
                          Цена в месяц
                          <input
                            type="number"
                            min="0"
                            value={plan.monthlyPrice}
                            onChange={(e) =>
                              setPlans((current) =>
                                current.map((x) =>
                                  x.code === plan.code
                                    ? {
                                        ...x,
                                        monthlyPrice: Number(e.target.value),
                                      }
                                    : x,
                                ),
                              )
                            }
                          />
                        </label>
                        <label>
                          Статус
                          <select
                            value={plan.status}
                            onChange={(e) =>
                              setPlans((current) =>
                                current.map((x) =>
                                  x.code === plan.code
                                    ? { ...x, status: e.target.value }
                                    : x,
                                ),
                              )
                            }
                          >
                            <option value="active">Активен</option>
                            <option value="archived">Скрыт</option>
                          </select>
                        </label>
                        <button onClick={() => void savePlan(plan)}>
                          Сохранить
                        </button>
                      </article>
                    ))}
                  </div>
                </section>
              )}
              {view === "insights" && (
                <section className="platform-insight-grid">
                  <article>
                    <strong>{platformAnalytics.visits}</strong>
                    <span>Всего посещений</span>
                  </article>
                  <article>
                    <strong>{platformAnalytics.newThisMonth}</strong>
                    <span>Новых за месяц</span>
                  </article>
                  <article>
                    <strong>{platformAnalytics.frequent}</strong>
                    <span>Частых гостей</span>
                  </article>
                  <article>
                    <strong>{platformAnalytics.loyal}</strong>
                    <span>Постоянных клиентов</span>
                  </article>
                  <article className="risk">
                    <strong>{platformAnalytics.atRisk}</strong>
                    <span>В риске ухода</span>
                  </article>
                </section>
              )}
              <section className="platform-card platform-task-list">
                <div className="platform-card-head">
                  <div>
                    <h2>Рабочая область</h2>
                    <p>Компании и доступные операции</p>
                  </div>
                </div>
                {companies.slice(0, 5).map((company) => (
                  <button key={company.id} onClick={() => setView("companies")}>
                    <span>
                      <CheckCircle2 />
                    </span>
                    <div>
                      <strong>{company.name}</strong>
                      <small>
                        {company.plan} · {company.status} · {company.customers}{" "}
                        клиентов
                      </small>
                    </div>
                    <ChevronRight />
                  </button>
                ))}
              </section>
            </div>
          )}
        </section>
      </main>
      {open && (
        <CompanyProvisioningWizard
          request={request}
          onClose={() => setOpen(false)}
          onCreated={async (data) => {
            setOpen(false);
            setMsg(`Компания создана. Guest URL: ${data.guestUrl}`);
            await load();
          }}
        />
      )}
      {selected && (
        <div
          className="platform-company-modal"
          onMouseDown={(e) => e.target === e.currentTarget && setSelected(null)}
        >
          <section>
            <header>
              <div>
                <small>КОМПАНИЯ</small>
                <h2>{selected.name}</h2>
                <p>
                  {selected.slug} · {selected.plan}
                </p>
              </div>
              <button onClick={() => setSelected(null)} aria-label="Закрыть">
                ×
              </button>
            </header>
            <div className="company-admin-metrics">
              <span>
                <small>Клиенты</small>
                <strong>{selected.customers}</strong>
              </span>
              <span>
                <small>Статус</small>
                <strong>{selected.status}</strong>
              </span>
            </div>
            <label className="company-plan-select">
              Тариф
              <select
                defaultValue={
                  plans.find(
                    (p) => p.name.toLowerCase() === selected.plan.toLowerCase(),
                  )?.code || plans[0]?.code
                }
                onChange={(e) => void changeCompanyPlan(e.target.value)}
              >
                {plans
                  .filter((p) => p.status === "active")
                  .map((plan) => (
                    <option value={plan.code} key={plan.code}>
                      {plan.name} · {plan.monthlyPrice.toLocaleString("ru-RU")}{" "}
                      ₸
                    </option>
                  ))}
              </select>
              <small>Выбор сразу создаёт новую активную подписку</small>
            </label>
            <div className="company-admin-actions">
              <button onClick={() => void changeStatus("active")}>
                Активировать
              </button>
              <button
                className="danger"
                onClick={() => void changeStatus("blocked")}
              >
                Заблокировать
              </button>
              <button
                onClick={() => {
                  setSelected(null);
                  setView("revenue");
                }}
              >
                Подписка и тариф
              </button>
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
