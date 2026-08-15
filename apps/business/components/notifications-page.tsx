"use client";
import { FormEvent, useEffect, useState } from "react";
import { Bell, Send } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { Notice } from "./management-shared";

type Notification = {
  id: string;
  recipient: string;
  subject: string;
  body: string;
  status: string;
  createdAt: string;
};
export function NotificationsPage() {
  const [items, setItems] = useState<Notification[]>([]);
  const [open, setOpen] = useState(false);
  const [msg, setMsg] = useState("");
  const load = () =>
    api<Notification[]>("/notifications")
      .then(setItems)
      .catch((e) => setMsg(e.message));
  useEffect(() => {
    void load();
  }, []);
  async function send(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    try {
      await api("/notifications/send", {
        method: "POST",
        body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
      });
      setOpen(false);
      setMsg("Письмо отправлено");
      await load();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка отправки");
    }
  }
  return (
    <SectionShell
      active="/notifications"
      title="Рассылки"
      subtitle="Email-уведомления клиентам"
    >
      <Notice text={msg} />
      <div className="toolbar">
        <a href="http://localhost:8025" target="_blank">
          Открыть локальный почтовый ящик
        </a>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Send />
          Новое письмо
        </button>
      </div>
      <div className="data-card">
        <table>
          <thead>
            <tr>
              <th>Получатель</th>
              <th>Тема</th>
              <th>Статус</th>
              <th>Дата</th>
            </tr>
          </thead>
          <tbody>
            {items.map((x) => (
              <tr key={x.id}>
                <td>{x.recipient}</td>
                <td>
                  <strong>{x.subject}</strong>
                </td>
                <td>
                  <span
                    className={
                      x.status === "sent" ? "mail-sent" : "mail-failed"
                    }
                  >
                    {x.status}
                  </span>
                </td>
                <td>{new Date(x.createdAt).toLocaleString("ru-RU")}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!items.length && (
          <div className="zero">
            <Bell />
            <strong>Писем пока нет</strong>
          </div>
        )}
      </div>
      {open && (
        <div className="sheet-bg">
          <form className="sheet" onSubmit={send}>
            <h2>Новое письмо</h2>
            <label>
              Email получателя
              <input name="recipient" type="email" required />
            </label>
            <label>
              Тема
              <input name="subject" required />
            </label>
            <label>
              Сообщение
              <textarea name="body" rows={7} required />
            </label>
            <div>
              <button type="button" onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button className="primary-action">Отправить</button>
            </div>
          </form>
        </div>
      )}
    </SectionShell>
  );
}
