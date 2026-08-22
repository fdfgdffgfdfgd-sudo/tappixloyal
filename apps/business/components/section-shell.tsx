"use client";
import Image from "next/image";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import {
  BarChart3,
  Building2,
  Camera,
  Check,
  ChevronDown,
  Gift,
  FileText,
  Menu,
  LayoutDashboard,
  LogOut,
  Plug,
  Search,
  Send,
  Settings,
  ShieldAlert,
  Users,
  UsersRound,
  X,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { api, logout } from "@/lib/api";
import { workspaceRoleLabel } from "@/lib/labels";
import { PageHeader } from "./ui-system";
import { useDialogFocusTrap } from "./use-dialog-focus-trap";

const items: readonly (readonly [string, readonly (readonly [string, LucideIcon, string])[]])[] = [
  ["Работа", [["/", LayoutDashboard, "Обзор"], ["/customers", Users, "Клиенты"], ["/scanner", Camera, "Staff Mode"]]],
  ["Лояльность", [["/loyalty", Gift, "Программа"], ["/referrals", UsersRound, "Рефералы"]]],
  ["Коммуникации", [["/campaigns", Send, "Кампании и автоматизации"]]],
  ["Аналитика", [["/analytics", BarChart3, "Аналитика"], ["/reports", FileText, "Отчёты"]]],
  ["Система", [["/integrations", Plug, "Интеграции"], ["/risk-center", ShieldAlert, "Проверка операций"], ["/employees", Users, "Команда"], ["/settings", Settings, "Настройки"]]],
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
type Workspace = {
  id: string;
  name: string;
  slug: string;
  role: string;
  plan: string;
  current: boolean;
};
type Identity = {
  role: string;
  firstName: string;
  lastName: string;
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
  className,
  children,
}: {
  active: string;
  title: string;
  subtitle: string;
  className?: string;
  children: React.ReactNode;
}) {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [menuOpen, setMenuOpen] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  const [commandQuery, setCommandQuery] = useState("");
  const [identity, setIdentity] = useState<Identity | null>(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [unreachable, setUnreachable] = useState(false);
  const commandTrigger = useRef<HTMLButtonElement>(null);
  const commandDialog = useDialogFocusTrap<HTMLDivElement>(commandOpen, closeCommand);
  useEffect(() => {
    Promise.all([
      api<Workspace[]>("/workspaces"),
      api<Identity>("/auth/me"),
    ])
      .then(([spaces, who]) => {
        setWorkspaces(spaces);
        setIdentity(who);
        setUnreachable(false);
      })
      // Swallowing this used to leave the header showing a placeholder name and
      // the sidebar claiming the system was fine. Say so instead.
      .catch(() => setUnreachable(true))
      .finally(() => setCheckingSession(false));
  }, []);
  const fullName = identity ? [identity.firstName, identity.lastName].filter(Boolean).join(" ") : "";
  // The shortcut listener is bound once, so it reads the palette's state here
  // rather than from a stale closure.
  const commandOpenRef = useRef(commandOpen);
  useEffect(() => {
    commandOpenRef.current = commandOpen;
  }, [commandOpen]);
  useEffect(() => {
    function shortcut(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setCommandOpen(true);
      }
      if (event.key === "Escape") {
        setNavOpen(false);
        // Only the palette's own Escape may move focus. Restoring it on every
        // Escape pulled focus to the search button out of whatever dialog the
        // page had just closed.
        if (commandOpenRef.current) closeCommand();
      }
    }
    window.addEventListener("keydown", shortcut);
    return () => window.removeEventListener("keydown", shortcut);
  }, []);
  const current =
    workspaces.find((workspace) => workspace.current) || workspaces[0];
  const selected = settingsRoutes.has(active) && active !== "/employees" ? "/settings" : active;
  // Until the role is confirmed, show only the group both roles share. Rendering
  // the full owner navigation first briefly offered an employee links they
  // cannot use.
  const visibleItems = !identity ? [items[0]] : identity.role === "employee"
    ? [["Работа", [["/scanner", Camera, "Staff Mode"]]]] as const
    : items;
  const commandItems = visibleItems
    .flatMap(([, links]) => links)
    .filter(([, , label]) => label.toLocaleLowerCase("ru-RU").includes(commandQuery.trim().toLocaleLowerCase("ru-RU")));
  function closeCommand() {
    setCommandOpen(false);
    setCommandQuery("");
    requestAnimationFrame(() => commandTrigger.current?.focus());
  }
  useEffect(() => {
    // Keep employees inside the operational surface even when they paste an
    // owner URL directly. The API remains the final authorization boundary.
    if (identity?.role === "employee" && active !== "/scanner") {
      window.location.replace("/scanner");
    }
  }, [identity, active]);
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
    // The profile is read back from /auth/me after the reload, so there is no
    // cached copy to keep in sync here.
    window.location.assign("/");
  }
  if (checkingSession) {
    return <main className="session-check" aria-busy="true" aria-live="polite"><span className="session-check-mark">T</span><strong>Проверяем доступ…</strong></main>;
  }
  if (!identity) {
    return <main className="session-check session-check-error"><strong>Не удалось подтвердить вход</strong><p>Войдите снова или повторите проверку.</p><div><a href="/login">Войти</a><button onClick={() => location.reload()}>Повторить</button></div></main>;
  }
  return (
    <div className={`product-shell role-${identity.role}`} data-role={identity.role}>
      <a className="skip" href="#main-content">К содержанию</a>
      <aside className={`product-sidebar ${navOpen ? "is-open" : ""}`}>
        <div className="product-brand"><Image className="product-mark" src="/tappix-mark.png" alt="" aria-hidden width={30} height={30} priority/><strong>Tappix</strong><button aria-label="Закрыть меню" onClick={() => setNavOpen(false)}><X/></button></div>
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
                    <small>{workspaceRoleLabel(workspace.role)}</small>
                  </span>
                  {workspace.current && <i aria-label="Текущее пространство"><Check /></i>}
                </button>
              ))}
              {identity.role !== "employee" && <>
                <div className="workspace-menu-divider" />
                <Link href="/settings">
                  <Building2 />
                  Настройки компании
                </Link>
                <Link href="/employees">
                  <Users />
                  Пригласить сотрудников
                </Link>
              </>}
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
          {visibleItems.map(([group, links]) => <div className="product-nav-group is-open" key={group}><span className="product-nav-label">{group}</span><div>{links.map(([href, Icon, label]) => (
            <Link aria-current={selected === href ? "page" : undefined} className={selected === href ? "current" : ""} href={href} key={href} onClick={() => setNavOpen(false)}><Icon />{label}</Link>
          ))}</div></div>)}
        </nav>
        <div className={`sidebar-meta ${unreachable ? "is-offline" : ""}`} role="status">
          <span className={unreachable ? "offline-dot" : identity ? "online-dot" : "pending-dot"} />
          {unreachable ? "Нет связи с сервером" : identity ? "Система работает" : "Проверяем соединение…"}
        </div>
      </aside>
      <main className={`product-main ${className || ""}`.trim()}>
        <PageHeader eyebrow={current?.name || "Рабочее пространство"} title={title} subtitle={subtitle} leading={<button className="product-menu-toggle" aria-label="Открыть меню" onClick={() => setNavOpen(true)}><Menu/></button>} actions={<div className="product-user">
            <button ref={commandTrigger} className="product-command" onClick={() => setCommandOpen(true)}><Search/><span>Найти</span><kbd>⌘ K</kbd></button>
            <Link className="header-scanner" href="/scanner"><Camera/>Сканер</Link>
            {identity && <><span>{fullName.slice(0, 1)}</span>
            <div>
              <strong>{fullName}</strong>
              <small>{identity.role === "company_owner" ? "Владелец" : "Сотрудник"}</small>
            </div></>}
          </div>}/>
        <section id="main-content">{children}</section>
      </main>
      {navOpen && <button className="product-nav-scrim" aria-label="Закрыть меню" onClick={() => setNavOpen(false)}/>}
      {commandOpen && <div className="command-backdrop" role="presentation" onMouseDown={closeCommand}><div ref={commandDialog} className="command-panel" role="dialog" aria-modal="true" aria-label="Быстрый переход" onMouseDown={event => event.stopPropagation()}><label><Search/><input autoFocus value={commandQuery} onChange={event => setCommandQuery(event.target.value)} placeholder="Куда перейти?" aria-label="Поиск раздела"/><kbd>Esc</kbd></label><div>{commandItems.map(([href, Icon, label]) => <Link href={href} key={href} onClick={closeCommand}><Icon/><span>{label}</span></Link>)}{!commandItems.length && <p className="command-empty">Раздел не найден</p>}</div></div></div>}
    </div>
  );
}
