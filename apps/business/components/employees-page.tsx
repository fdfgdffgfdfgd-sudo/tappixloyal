"use client";
import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { Branch, Notice } from "./management-shared";
import { useConfirm } from "./use-confirm";

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
    {dialog}</SectionShell>
  );
}
