"use client";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  BarChart3,
  Building2,
  Camera,
  Check,
  ChevronDown,
  CreditCard,
  Gift,
  Globe2,
  Menu,
  LayoutDashboard,
  LogOut,
  Nfc,
  Plug,
  Search,
  Send,
  Settings,
  Star,
  Users,
  UsersRound,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { api, logout } from "@/lib/api";

const items: readonly (readonly [string, readonly (readonly [string, LucideIcon, string])[]])[] = [
  ["Работа", [["/", LayoutDashboard, "Обзор"], ["/customers", Users, "Клиенты"], ["/scanner", Camera, "Staff Mode"]]],
  ["Рост", [["/loyalty", Gift, "Лояльность"], ["/campaigns", Send, "Кампании"], ["/referrals", UsersRound, "Рефералы"], ["/reviews", Star, "Отзывы"], ["/analytics", BarChart3, "Аналитика"]]],
  ["Система", [["/devices", Nfc, "NFC и QR"], ["/integrations", Plug, "Интеграции"], ["/website", Globe2, "Сайт"], ["/subscription", CreditCard, "Тариф"], ["/settings", Settings, "Настройки"]]],
] as const;
const settingsRoutes = new Set([
  "/branches",
  "/employees",
  "/bookings",
  "/api-keys",
  "/files",
  "/modules",
  "/audit",
]);
const loyaltyRoutes = new Set(["/notifications"]);
type Workspace = {
  id: string;
  name: string;
  slug: string;
  role: string;
  plan: string;
  current: boolean;
};
type SwitchResult = {
  accessToken: string;
  refreshToken: string;
  companyId: string;
  role: string;
};

export function SectionShell({
  active,
  title,
  subtitle,
  children,
}: {
  active: string;
  title: string;
  subtitle: string;
  children: React.ReactNode;
}) {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [menuOpen, setMenuOpen] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  const [commandQuery, setCommandQuery] = useState("");
  const [actualRole, setActualRole] = useState("");
  const commandTrigger = useRef<HTMLButtonElement>(null);
  const user = useMemo(() => {
    if (typeof window === "undefined")
      return { firstName: "", lastName: "", role: "" };
    try {
      return JSON.parse(sessionStorage.getItem("tappix_user") || "{}");
    } catch {
      return {};
    }
  }, []);
  useEffect(() => {
    Promise.all([
      api<Workspace[]>("/workspaces"),
      api<{ role: string }>("/auth/me"),
    ])
      .then(([spaces, identity]) => {
        setWorkspaces(spaces);
        setActualRole(identity.role);
      })
      .catch(() => undefined);
  }, []);
  useEffect(() => {
    function shortcut(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setCommandOpen(true);
      }
      if (event.key === "Escape") {
        setCommandOpen(false);
        setCommandQuery("");
        requestAnimationFrame(() => commandTrigger.current?.focus());
        setNavOpen(false);
      }
    }
    window.addEventListener("keydown", shortcut);
    return () => window.removeEventListener("keydown", shortcut);
  }, []);
  const current =
    workspaces.find((workspace) => workspace.current) || workspaces[0];
  const selected = settingsRoutes.has(active)
    ? "/settings"
    : loyaltyRoutes.has(active)
      ? "/loyalty"
      : active;
  const commandItems = items
    .flatMap(([, links]) => links)
    .filter(([, , label]) => label.toLocaleLowerCase("ru-RU").includes(commandQuery.trim().toLocaleLowerCase("ru-RU")));
  function closeCommand() {
    setCommandOpen(false);
    setCommandQuery("");
    requestAnimationFrame(() => commandTrigger.current?.focus());
  }
  async function switchWorkspace(id: string) {
    if (id === current?.id) {
      setMenuOpen(false);
      return;
    }
    const result = await api<SwitchResult>(`/workspaces/${id}/switch`, {
      method: "POST",
    });
    sessionStorage.setItem("tappix_access", result.accessToken);
    sessionStorage.setItem("tappix_refresh", result.refreshToken);
    const saved = { ...user, companyId: result.companyId, role: result.role };
    sessionStorage.setItem("tappix_user", JSON.stringify(saved));
    window.location.assign("/");
  }
  return (
    <div className="product-shell">
      <a className="skip" href="#main-content">К содержанию</a>
      <aside className={`product-sidebar ${navOpen ? "is-open" : ""}`}>
        <div className="product-brand"><span>T</span><strong>Tappix</strong><button aria-label="Закрыть меню" onClick={() => setNavOpen(false)}><X/></button></div>
        <div className="workspace-control">
          <button
            aria-expanded={menuOpen}
            aria-haspopup="menu"
            onClick={() => setMenuOpen(!menuOpen)}
          >
            <span className="workspace-logo">
              {(current?.name || "T").slice(0, 1)}
            </span>
            <span>
              <strong>{current?.name || "Tappix"}</strong>
              <small>{current?.plan || "Workspace"}</small>
            </span>
            <ChevronDown />
          </button>
          {menuOpen && (
            <div className="workspace-menu" role="menu">
              <small>Рабочие пространства</small>
              {workspaces.map((workspace) => (
                <button
                  role="menuitem"
                  key={workspace.id}
                  onClick={() => void switchWorkspace(workspace.id)}
                >
                  <span className="workspace-logo">
                    {workspace.name.slice(0, 1)}
                  </span>
                  <span>
                    <strong>{workspace.name}</strong>
                    <small>{workspace.role}</small>
                  </span>
                  {workspace.current && <i aria-label="Текущее пространство"><Check /></i>}
                </button>
              ))}
              <div className="workspace-menu-divider" />
              <Link href="/settings">
                <Building2 />
                Настройки компании
              </Link>
              <Link href="/employees">
                <Users />
                Пригласить сотрудников
              </Link>
              <button
                className="workspace-logout"
                onClick={() => void logout()}
              >
                <LogOut />
                Выйти
              </button>
            </div>
          )}
        </div>
        <nav aria-label="Основная навигация">
          {items.map(([group, links]) => <div className="product-nav-group" key={group}><small>{group}</small>{links.map(([href, Icon, label]) => (
            <Link className={selected === href ? "current" : ""} href={href} key={href} onClick={() => setNavOpen(false)}><Icon />{label}</Link>
          ))}</div>)}
        </nav>
        <div className="sidebar-meta">
          <span className="online-dot" />
          Система работает
        </div>
      </aside>
      <main className="product-main">
        <header>
          <button className="product-menu-toggle" aria-label="Открыть меню" onClick={() => setNavOpen(true)}><Menu/></button>
          <div>
            <span className="product-eyebrow">
              {current?.name || "Рабочее пространство"}
            </span>
            <h1>{title}</h1>
            <p>{subtitle}</p>
          </div>
          <div className="product-user">
            <button ref={commandTrigger} className="product-command" onClick={() => setCommandOpen(true)}><Search/><span>Найти</span><kbd>⌘ K</kbd></button>
            <Link className="header-scanner" href="/scanner"><Camera/>Сканер</Link>
            <span>{(user.firstName || "А").slice(0, 1)}</span>
            <div>
              <strong>
                {[user.firstName, user.lastName].filter(Boolean).join(" ") ||
                  "Армат"}
              </strong>
              <small>
                {(actualRole || user.role) === "company_owner"
                  ? "Владелец"
                  : "Сотрудник"}
              </small>
            </div>
          </div>
        </header>
        <section id="main-content">{children}</section>
      </main>
      {navOpen && <button className="product-nav-scrim" aria-label="Закрыть меню" onClick={() => setNavOpen(false)}/>}
      {commandOpen && <div className="command-backdrop" role="presentation" onMouseDown={closeCommand}><div className="command-panel" role="dialog" aria-modal="true" aria-label="Быстрый переход" onMouseDown={event => event.stopPropagation()}><label><Search/><input autoFocus value={commandQuery} onChange={event => setCommandQuery(event.target.value)} placeholder="Куда перейти?" aria-label="Поиск раздела"/><kbd>Esc</kbd></label><div>{commandItems.map(([href, Icon, label]) => <Link href={href} key={href} onClick={closeCommand}><Icon/><span>{label}</span></Link>)}{!commandItems.length && <p className="command-empty">Раздел не найден</p>}</div></div></div>}
    </div>
  );
}
