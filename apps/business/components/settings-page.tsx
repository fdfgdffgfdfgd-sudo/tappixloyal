"use client";
import { FormEvent, useEffect, useState } from "react";
import { ShieldCheck } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { Notice } from "./management-shared";
import { OwnerContext } from "./owner-ux-primitives";

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
      <OwnerContext label="РАБОЧЕЕ ПРОСТРАНСТВО" title="Настройки бизнеса" detail="Соберите компанию, безопасность и оплату в понятные разделы." href="/subscription" action="Тариф и оплата" />
      <div className="settings-layout"><nav className="settings-nav" aria-label="Разделы настроек"><a className="active" href="#company">Компания</a><a href="#security">Безопасность</a><a href="/subscription">Тариф и оплата</a></nav><div className="settings-sections"><form id="company" className="settings-card settings-section" onSubmit={save}><header><div><small>КОМПАНИЯ</small><h2>Как клиенты видят бизнес</h2><p>Эти данные используются в карте клиента и уведомлениях.</p></div></header><div className="form-grid">
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
        </div><footer><button className="primary-action">Сохранить изменения</button></footer></form>
      <section id="security" className="settings-card session-card settings-section">
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
      </section></div></div>
    </SectionShell>
  );
}
