"use client";
import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { Branch, Notice } from "./management-shared";
import { useConfirm } from "./use-confirm";
import { StaffStatus } from "./owner-ux-primitives";

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
  const { ask, dialog } = useConfirm();
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
    if (!await ask({ title: "Удалить сотрудника?", confirmLabel: "Удалить" })) return;
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
      <section className="team-command-bar"><div><small>КОМАНДА</small><h2>{items.length} сотрудников имеют доступ</h2><p>Роль и филиал определяют, что видит каждый человек в Tappix.</p></div><button className="primary-action" onClick={() => setOpen(true)}><Plus />Добавить сотрудника</button></section>
      <section className="team-list" aria-label="Список сотрудников">
        <header><span>Сотрудник</span><span>Роль</span><span>Филиал</span><span>Доступ</span><span /></header>
        {items.length === 0 && <div className="team-empty"><strong>Команда пока пустая</strong><p>Добавьте первого сотрудника, чтобы работать с клиентами у кассы.</p><button className="primary-action" onClick={() => setOpen(true)}>Добавить сотрудника</button></div>}
        {items.map((x) => <article className="team-member" key={x.id}><div className="team-member-identity"><span>{x.firstName.slice(0, 1)}</span><div><strong>{x.firstName} {x.lastName}</strong><small>{x.email}</small></div></div><span className="tag">{x.role === "employee" ? "Сотрудник" : x.role}</span><span>{x.branch || "Все филиалы"}</span><StaffStatus active={x.status === "active"} /><div className="row-actions">{x.role === "employee" && <><button aria-label="Редактировать сотрудника" onClick={() => setEditing(x)}><Pencil /></button><button aria-label="Удалить сотрудника" className="danger-icon" onClick={() => remove(x.id)}><Trash2 /></button></>}</div></article>)}
      </section>
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
    {dialog}</SectionShell>
  );
}
