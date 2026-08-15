"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  CalendarClock,
  Download,
  FileSpreadsheet,
  Mail,
  Play,
  Plus,
  RefreshCw,
  Trash2,
  X,
} from "lucide-react";
import { api, download } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { useDialogFocusTrap } from "./use-dialog-focus-trap";
import { useConfirm } from "./use-confirm";

type Schedule = {
  id: string;
  name: string;
  channel: string;
  recipients: string[];
  frequency: string;
  timezone: string;
  sendHour: number;
  sendWeekday?: number;
  sendMonthday?: number;
  format: string;
  active: boolean;
  nextRunAt?: string;
  lastStatus?: string;
  lastError?: string;
};
type Run = {
  id: string;
  scheduleId: string;
  name: string;
  status: "queued" | "processing" | "sent" | "skipped" | "failed";
  format: string;
  filename: string;
  attempts: number;
  error: string;
  createdAt: string;
  nextAttemptAt?: string;
  downloadable: boolean;
};
type Draft = {
  name: string;
  channel: string;
  recipient: string;
  frequency: string;
  sendHour: number;
  format: string;
  active: boolean;
};
const emptyDraft: Draft = {
  name: "Отчёт владельцу",
  channel: "email",
  recipient: "",
  frequency: "weekly",
  sendHour: 9,
  format: "pdf",
  active: true,
};
const frequencyLabel: Record<string, string> = {
  daily: "Каждый день",
  weekly: "Каждую неделю",
  monthly: "Каждый месяц",
};
const statusLabel: Record<string, string> = {
  queued: "В очереди",
  processing: "Формируется",
  sent: "Отправлен",
  skipped: "Нужна настройка",
  failed: "Ошибка",
};

export function ReportsPage() {
  const { ask, dialog } = useConfirm();
  const [schedules, setSchedules] = useState<Schedule[]>([]),
    [runs, setRuns] = useState<Run[]>([]),
    [loading, setLoading] = useState(true),
    [open, setOpen] = useState(false),
    [saving, setSaving] = useState(false),
    [draft, setDraft] = useState<Draft>(emptyDraft),
    [message, setMessage] = useState(""),
    [error, setError] = useState("");
  const reportDialog = useDialogFocusTrap<HTMLFormElement>(open, () => setOpen(false));
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [next, history] = await Promise.all([
        api<Schedule[]>("/reports/schedules"),
        api<Run[]>("/reports/runs"),
      ]);
      setSchedules(next);
      setRuns(history);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Не удалось загрузить отчёты",
      );
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  async function create(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      await api("/reports/schedules", {
        method: "POST",
        body: JSON.stringify({
          name: draft.name,
          reportType: "owner_summary",
          channel: draft.channel,
          recipients: [draft.recipient],
          frequency: draft.frequency,
          timezone: "Asia/Almaty",
          sendHour: draft.sendHour,
          sendWeekday: draft.frequency === "weekly" ? 1 : null,
          sendMonthday: draft.frequency === "monthly" ? 1 : null,
          format: draft.format,
          active: draft.active,
        }),
      });
      setOpen(false);
      setDraft(emptyDraft);
      setMessage("Расписание отчёта создано");
      await load();
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Не удалось создать расписание",
      );
    } finally {
      setSaving(false);
    }
  }
  async function run(id: string) {
    setMessage("");
    setError("");
    try {
      await api(`/reports/schedules/${id}/run`, { method: "POST" });
      setMessage("Отчёт поставлен в очередь");
      setTimeout(() => void load(), 900);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Не удалось запустить отчёт",
      );
    }
  }
  async function retry(id: string) {
    try {
      await api(`/reports/runs/${id}/retry`, { method: "POST" });
      setMessage("Повтор поставлен в очередь");
      setTimeout(() => void load(), 900);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Не удалось повторить доставку",
      );
    }
  }
  async function remove(id: string) {
    if (
      !await ask({ title: "Удалить расписание?", description: "История его запусков также будет удалена.", confirmLabel: "Удалить" })
    )
      return;
    try {
      await api(`/reports/schedules/${id}`, { method: "DELETE" });
      setMessage("Расписание удалено");
      await load();
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Не удалось удалить расписание",
      );
    }
  }
  return (
    <SectionShell
      active="/reports"
      title="Регулярные отчёты"
      subtitle="Получайте главные показатели бизнеса без входа в Tappix"
    >
      {message && (
        <div className="notice" role="status">
          {message}
        </div>
      )}
      {error && (
        <div className="report-error" role="alert">
          <span>{error}</span>
          <button onClick={() => void load()}>
            <RefreshCw />
            Повторить
          </button>
        </div>
      )}
      <section className="report-intro">
        <div>
          <span>
            <FileSpreadsheet />
          </span>
          <div>
            <small>ОТЧЁТ ВЛАДЕЛЬЦУ</small>
            <h2>Цифры приходят сами</h2>
            <p>
              Клиенты, повторные покупки, выручка участников, обязательства по
              бонусам и потерянные гости — в одном файле.
            </p>
          </div>
        </div>
        <button className="primary-action" onClick={() => setOpen(true)}>
          <Plus />
          Создать расписание
        </button>
      </section>
      <div className="report-layout">
        <section className="report-panel">
          <header>
            <div>
              <h2>Расписания</h2>
              <p>Когда и кому отправлять отчёт</p>
            </div>
          </header>
          {loading ? (
            <div className="report-skeleton" aria-label="Загрузка расписаний" />
          ) : schedules.length ? (
            schedules.map((item) => (
              <article className="schedule-row" key={item.id}>
                <span>
                  <CalendarClock />
                </span>
                <div>
                  <strong>{item.name}</strong>
                  <small>
                    {frequencyLabel[item.frequency]} в{" "}
                    {String(item.sendHour).padStart(2, "0")}:00 ·{" "}
                    {item.format.toUpperCase()} · {item.recipients.join(", ")}
                  </small>
                  {item.lastError && <em>{item.lastError}</em>}
                </div>
                <b className={`report-status ${item.lastStatus || "ready"}`}>
                  {item.lastStatus ? statusLabel[item.lastStatus] : "Готово"}
                </b>
                <button
                  aria-label={`Сформировать ${item.name}`}
                  onClick={() => void run(item.id)}
                >
                  <Play />
                </button>
                <button
                  aria-label={`Удалить ${item.name}`}
                  onClick={() => void remove(item.id)}
                >
                  <Trash2 />
                </button>
              </article>
            ))
          ) : (
            <div className="report-empty">
              <Mail />
              <h3>Отчёты ещё не настроены</h3>
              <p>
                Создайте одно расписание — Tappix будет сам собирать показатели
                и отправлять владельцу.
              </p>
              <button className="primary-action" onClick={() => setOpen(true)}>
                <Plus />
                Создать первый отчёт
              </button>
            </div>
          )}
        </section>
        <section className="report-panel">
          <header>
            <div>
              <h2>Последние отправки</h2>
              <p>Файл, статус и причина ошибки</p>
            </div>
            <button
              className="report-refresh"
              aria-label="Обновить историю"
              onClick={() => void load()}
            >
              <RefreshCw />
            </button>
          </header>
          {loading ? (
            <div className="report-skeleton" />
          ) : runs.length ? (
            runs.map((item) => (
              <article className="run-row" key={item.id}>
                <div>
                  <strong>{item.name}</strong>
                  <small>
                    {new Date(item.createdAt).toLocaleString("ru-RU")} · попытка{" "}
                    {item.attempts}
                  </small>
                  {item.error && <em>{item.error}</em>}
                  {item.status === "queued" && item.attempts > 0 && item.nextAttemptAt && (
                    <em>Повторим автоматически {new Date(item.nextAttemptAt).toLocaleString("ru-RU")}</em>
                  )}
                </div>
                <b className={`report-status ${item.status}`}>
                  {statusLabel[item.status]}
                </b>
                {item.downloadable && (
                  <button
                    aria-label="Скачать отчёт"
                    onClick={() =>
                      void download(
                        `/reports/runs/${item.id}/download`,
                        item.filename,
                      )
                    }
                  >
                    <Download />
                  </button>
                )}
                {(item.status === "failed" || item.status === "skipped") && (
                  <button onClick={() => void retry(item.id)}>Повторить</button>
                )}
              </article>
            ))
          ) : (
            <div className="report-empty compact">
              <FileSpreadsheet />
              <h3>История пока пуста</h3>
              <p>Здесь появятся ручные и регулярные отправки.</p>
            </div>
          )}
        </section>
      </div>
      {open && (
        <div
          className="report-dialog-bg"
          role="presentation"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) setOpen(false);
          }}
        >
          <form
            ref={reportDialog}
            className="report-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="report-dialog-title"
            onSubmit={create}
          >
            <header>
              <div>
                <small>НОВОЕ РАСПИСАНИЕ</small>
                <h2 id="report-dialog-title">Отчёт для владельца</h2>
                <p>Настройте один раз — дальше Tappix всё сделает сам.</p>
              </div>
              <button
                type="button"
                aria-label="Закрыть"
                onClick={() => setOpen(false)}
              >
                <X />
              </button>
            </header>
            <label>
              Название
              <input
                required
                value={draft.name}
                onChange={(event) =>
                  setDraft({ ...draft, name: event.target.value })
                }
              />
            </label>
            <label>
              Получатель
              <input
                required
                type={draft.channel === "email" ? "email" : "text"}
                placeholder={
                  draft.channel === "email"
                    ? "owner@company.kz"
                    : "+7 700 000 00 00"
                }
                value={draft.recipient}
                onChange={(event) =>
                  setDraft({ ...draft, recipient: event.target.value })
                }
              />
            </label>
            <div className="report-form-grid">
              <label>
                Канал
                <select
                  value={draft.channel}
                  onChange={(event) =>
                    setDraft({ ...draft, channel: event.target.value })
                  }
                >
                  <option value="email">Email</option>
                  <option value="whatsapp">WhatsApp</option>
                </select>
              </label>
              <label>
                Частота
                <select
                  value={draft.frequency}
                  onChange={(event) =>
                    setDraft({ ...draft, frequency: event.target.value })
                  }
                >
                  <option value="daily">Ежедневно</option>
                  <option value="weekly">Еженедельно</option>
                  <option value="monthly">Ежемесячно</option>
                </select>
              </label>
              <label>
                Формат
                <select
                  value={draft.format}
                  onChange={(event) =>
                    setDraft({ ...draft, format: event.target.value })
                  }
                >
                  <option value="pdf">PDF</option>
                  <option value="xlsx">XLSX</option>
                  <option value="csv">CSV</option>
                  <option value="summary">Краткий текст</option>
                </select>
              </label>
              <label>
                Время
                <input
                  type="number"
                  min="0"
                  max="23"
                  value={draft.sendHour}
                  onChange={(event) =>
                    setDraft({ ...draft, sendHour: Number(event.target.value) })
                  }
                />
              </label>
            </div>
            {draft.channel === "whatsapp" && (
              <p className="report-callout">
                Для WhatsApp подключите Meta Cloud API и публичный HTTPS-адрес
                Tappix. Готовый файл придёт как защищённый документ; если
                подключение неполное, запуск будет отмечен как «Нужна настройка».
              </p>
            )}
            <footer>
              <button type="button" onClick={() => setOpen(false)}>
                Отмена
              </button>
              <button className="primary-action" disabled={saving}>
                {saving ? "Сохраняем…" : "Создать расписание"}
              </button>
            </footer>
          </form>
        </div>
      )}
    {dialog}</SectionShell>
  );
}
