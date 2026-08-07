"use client";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import {
  BarChart3,
  Building2,
  Camera,
  ChevronDown,
  Gift,
  LayoutDashboard,
  LogOut,
  Nfc,
  Settings,
  Star,
  Users,
} from "lucide-react";
import { api, logout } from "@/lib/api";

const items = [
  ["/", LayoutDashboard, "Обзор"],
  ["/customers", Users, "Клиенты"],
  ["/loyalty", Gift, "Лояльность"],
  ["/devices", Nfc, "NFC и QR"],
  ["/reviews", Star, "Отзывы"],
  ["/analytics", BarChart3, "Аналитика"],
  ["/settings", Settings, "Настройки"],
] as const;
const settingsRoutes = new Set([
  "/branches",
  "/employees",
  "/website",
  "/bookings",
  "/api-keys",
  "/files",
  "/integrations",
  "/modules",
  "/subscription",
  "/audit",
]);
const loyaltyRoutes = new Set(["/notifications", "/campaigns"]);
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
  const [actualRole, setActualRole] = useState("");
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
  const current =
    workspaces.find((workspace) => workspace.current) || workspaces[0];
  const selected = settingsRoutes.has(active)
    ? "/settings"
    : loyaltyRoutes.has(active)
      ? "/loyalty"
      : active;
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
      <aside className="product-sidebar">
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
                  {workspace.current && <i>✓</i>}
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
          {items.map(([href, Icon, label]) => (
            <Link
              className={selected === href ? "current" : ""}
              href={href}
              key={href}
            >
              <Icon />
              {label}
            </Link>
          ))}
        </nav>
        <div className="sidebar-meta">
          <span className="online-dot" />
          Система работает
        </div>
      </aside>
      <main className="product-main">
        <header>
          <div>
            <span className="product-eyebrow">
              {current?.name || "Рабочее пространство"}
            </span>
            <h1>{title}</h1>
            <p>{subtitle}</p>
          </div>
          <div className="product-user">
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
        <section>{children}</section>
      </main>
    </div>
  );
}
