"use client";
import { FormEvent, useEffect, useState } from "react";
import { Nfc, Pencil, Plus, QrCode, ToggleLeft, ToggleRight, Trash2 } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { Branch, Notice } from "./management-shared";

type Device = {
  id: string;
  branchId: string;
  branch: string;
  kind: string;
  name: string;
  url: string;
  destination: string;
  active: boolean;
  scans: number;
};
export function LegacyDevicesPage() {
  const [items, setItems] = useState<Device[]>([]);
  const [branches, setBranches] = useState<Branch[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<Device | null>(null);
  const [msg, setMsg] = useState("");
  const load = async () => {
    try {
      const [data, branchData] = await Promise.all([
        api<Device[]>("/devices"),
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
      await api("/devices", {
        method: "POST",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setOpen(false);
      setMsg("Устройство создано");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function save(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!editing) return;
    const data = Object.fromEntries(new FormData(e.currentTarget));
    try {
      await api(`/devices/${editing.id}`, {
        method: "PATCH",
        body: JSON.stringify({ ...data, active: editing.active }),
      });
      setEditing(null);
      setMsg("Устройство обновлено");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function toggle(item: Device) {
    try {
      await api(`/devices/${item.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          branchId: item.branchId,
          kind: item.kind,
          name: item.name,
          destination: item.destination,
          active: !item.active,
        }),
      });
      setMsg(item.active ? "Устройство отключено" : "Устройство включено");
      await load();
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function remove(id: string) {
    if (!confirm("Удалить устройство?")) return;
    await api(`/devices/${id}`, { method: "DELETE" });
    setMsg("Устройство удалено");
    await load();
  }
  const destinationOptions = (
    <>
      <option value="join">Регистрация клиента</option>
      <option value="reviews">Отзывы</option>
      <option value="website">Мини-сайт</option>
      <option value="booking">Онлайн-запись</option>
    </>
  );
  return (
    <SectionShell
      active="/devices"
      title="NFC и QR"
      subtitle="Точки регистрации клиентов"
    >
      <Notice text={msg} />
      <div className="toolbar">
        <span>{items.length} устройств</span>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Plus />
          Новое устройство
        </button>
      </div>
      <div className="card-grid">
        {items.map((x) => (
          <article className="entity-card" key={x.id}>
            <span className="entity-icon">
              {x.kind === "nfc" ? <Nfc /> : <QrCode />}
            </span>
            <div>
              <h2>{x.name}</h2>
              <p>
                {x.branch} · {x.scans} сканирований
              </p>
              <span className={x.active ? "status" : "tag"}>
                {x.active ? "Активно" : "Отключено"}
              </span>
              <a href={x.url} target="_blank" rel="noreferrer">
                {x.url}
              </a>
            </div>
            <div className="row-actions">
              <button
                aria-label="Редактировать устройство"
                onClick={() => setEditing(x)}
              >
                <Pencil />
              </button>
              <button
                aria-label={
                  x.active ? "Отключить устройство" : "Включить устройство"
                }
                onClick={() => toggle(x)}
              >
                {x.active ? <ToggleRight /> : <ToggleLeft />}
              </button>
              <button
                aria-label="Удалить устройство"
                className="danger-icon"
                onClick={() => remove(x.id)}
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
            <h2>Новое устройство</h2>
            <label>
              Тип
              <select name="kind">
                <option value="nfc">NFC</option>
                <option value="qr">QR</option>
              </select>
            </label>
            <label>
              Название
              <input name="name" required placeholder="Reception" />
            </label>
            <label>
              Филиал
              <select name="branchId" required>
                {branches.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Назначение<select name="destination">{destinationOptions}</select>
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
            <h2>Редактировать устройство</h2>
            <label>
              Тип
              <select name="kind" defaultValue={editing.kind}>
                <option value="nfc">NFC</option>
                <option value="qr">QR</option>
              </select>
            </label>
            <label>
              Название
              <input name="name" defaultValue={editing.name} required />
            </label>
            <label>
              Филиал
              <select name="branchId" defaultValue={editing.branchId} required>
                {branches.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Назначение
              <select name="destination" defaultValue={editing.destination}>
                {destinationOptions}
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
    </SectionShell>
  );
}
