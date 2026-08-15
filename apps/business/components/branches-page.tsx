"use client";
import { FormEvent, useEffect, useState } from "react";
import { Building2, Pencil, Plus, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Branch, Notice } from "./management-shared";

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
