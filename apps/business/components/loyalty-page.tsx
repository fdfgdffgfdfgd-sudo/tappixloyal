"use client";
import { FormEvent, useEffect, useState } from "react";
import { Bell, Cake, Gift, Nfc, Plus, QrCode, UserX, TrendingUp } from "lucide-react";
import { RewardBuilder } from "./reward-builder";
import { ProgramMechanics } from "./program-mechanics";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Loyalty, Notice } from "./management-shared";

export function LoyaltyPage() {
  const [value, setValue] = useState<Loyalty>({
    welcomeBonus: 0,
    pointsPerVisit: 20,
    birthdayBonus: 0,
    visitsForReward: 10,
    rewardName: "Подарок",
  });
  const [msg, setMsg] = useState("");
  const [inactiveDays, setInactiveDays] = useState(30);
  const [inactive, setInactive] = useState<
    {
      id: string;
      firstName: string;
      lastName: string;
      phone: string;
      lastVisitAt?: string;
    }[]
  >([]);
  const [processing, setProcessing] = useState(false);
  useEffect(() => {
    api<Loyalty>("/loyalty/rules")
      .then(setValue)
      .catch((e) => setMsg(e.message));
  }, []);
  async function save(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/loyalty/rules", {
        method: "PATCH",
        body: JSON.stringify(value),
      });
      setMsg("Правила лояльности сохранены");
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function loadInactive(days = inactiveDays) {
    try {
      const result = await api<{ items: typeof inactive }>(
        `/loyalty/inactive?days=${days}`,
      );
      setInactive(result.items);
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    }
  }
  async function runBirthdays() {
    setProcessing(true);
    try {
      const result = await api<{ processed: number }>(
        "/loyalty/process-birthdays",
        { method: "POST", body: "{}" },
      );
      setMsg(
        result.processed
          ? `Birthday-бонус начислен ${result.processed} клиентам`
          : "Все birthday-бонусы за сегодня уже обработаны",
      );
    } catch (error) {
      setMsg(error instanceof Error ? error.message : "Ошибка");
    } finally {
      setProcessing(false);
    }
  }
  useEffect(() => {
    void loadInactive();
  }, []);
  const field = (key: keyof Loyalty, label: string, type = "number") => (
    <label>
      {label}
      <input
        type={type}
        value={value[key]}
        onChange={(e) =>
          setValue({
            ...value,
            [key]: type === "number" ? Number(e.target.value) : e.target.value,
          })
        }
      />
    </label>
  );
  return (
    <SectionShell
      active="/loyalty"
      title="Программа лояльности"
      subtitle="Настройте начисления и награды без разработчика"
      className="loyalty-page-v11"
    >
      <Notice text={msg} />
      <header className="loyalty-workspace-header">
        <div className="loyalty-workspace-heading">
          <small>КОНСТРУКТОР ПРОГРАММЫ</small>
          <h2>За что клиент будет возвращаться?</h2>
          <p>Выберите один сценарий, настройте награду и сразу проверьте результат глазами клиента.</p>
        </div>
        <span className="loyalty-state-pill">Черновик</span>
      </header>
      <section className="loyalty-editor-rail"><span><b>1</b><strong>Механика</strong><small>За что начисляем прогресс</small></span><span><b>2</b><strong>Награда</strong><small>Что получит клиент</small></span><span><b>3</b><strong>Запуск</strong><small>QR, сотрудники и публикация</small></span></section>
      <div className="loyalty-config-layout"><div className="loyalty-workspace-panel"><ProgramMechanics />
        <section className="loyalty-launch-path"><header><div><small>ПОСЛЕ ПУБЛИКАЦИИ</small><h2>Три шага до первых результатов</h2></div></header><div className="loyalty-next-actions">
          <Link href="/devices"><b>1</b><QrCode/><span><strong>Разместите QR или NFC</strong><small>Клиент откроет и сохранит карту</small></span></Link>
          <Link href="/scanner"><b>2</b><Nfc/><span><strong>Отмечайте покупки</strong><small>Сотрудник сканирует карту гостя</small></span></Link>
          <Link href="/analytics"><b>3</b><TrendingUp/><span><strong>Следите за возвратом</strong><small>Увидите повторные визиты и выручку</small></span></Link>
        </div></section>
      </div><aside className="loyalty-live-preview"><small>КАК ЭТО УВИДИТ КЛИЕНТ</small><div className="loyalty-preview-card"><span className="preview-mark">T</span><strong>Ваша карта лояльности</strong><p>Посещения и награды в одном месте</p><div><b>1 из 6</b><span>до подарка</span></div><em>Показать карту на кассе</em></div><p className="preview-note">Изменения появятся на карте после публикации.</p></aside></div>
      <details className="loyalty-secondary-section"><summary><Gift/><span><strong>Дополнительные награды</strong><small>Скидки, услуги и подарки поверх основной программы</small></span><Plus/></summary><div><RewardBuilder /></div></details>
      <details className="loyalty-secondary-section"><summary><Bell/><span><strong>Дополнительные правила</strong><small>Приветственный бонус, день рождения и возврат клиентов</small></span><Plus/></summary><div><div className="workspace-explainer"><Bell/><div><small>АВТОМАТИЧЕСКИЕ ДЕЙСТВИЯ</small><h2>Система действует в нужный момент</h2><p>Нулевое значение отключает соответствующее начисление.</p></div></div>
      <form className="settings-card" onSubmit={save}>
        <div className="settings-title">
          <span>
            <Gift />
          </span>
          <div>
            <h2>Автоматические начисления</h2>
            <p>Применяются ко всем филиалам. Нулевое значение отключает начисление.</p>
          </div>
        </div>
        <div className="form-grid">
          {field("welcomeBonus", "После регистрации, бонусов")}
          {field("pointsPerVisit", "После посещения, бонусов")}
          {field("birthdayBonus", "На день рождения, бонусов")}
          {field("visitsForReward", "Цель по посещениям")}
          {field("rewardName", "Подарок за достижение цели", "text")}
        </div>
        <button className="primary-action">Сохранить правила</button>
      </form>
      <div className="loyalty-automation-grid">
        <section className="automation-card">
          <span>
            <Cake />
          </span>
          <div>
            <h2>День рождения</h2>
            <p>
              Система начисляет настроенный бонус один раз в год. Кнопка запускает безопасную ручную проверку прямо сейчас.
            </p>
          </div>
          <button
            disabled={processing || value.birthdayBonus <= 0}
            onClick={runBirthdays}
          >
            {processing ? "Проверяем…" : value.birthdayBonus <= 0 ? "Сначала задайте бонус выше" : "Проверить сейчас"}
          </button>
        </section>
        <section className="inactive-card">
          <div className="inactive-title">
            <span>
              <UserX />
            </span>
            <div>
              <h2>Пора вернуть</h2>
              <p>Клиенты без визитов за выбранный период</p>
            </div>
            <select
              aria-label="Период неактивности"
              value={inactiveDays}
              onChange={(e) => {
                const days = Number(e.target.value);
                setInactiveDays(days);
                void loadInactive(days);
              }}
            >
              <option value={30}>30 дней</option>
              <option value={60}>60 дней</option>
              <option value={90}>90 дней</option>
            </select>
          </div>
          <strong>{inactive.length}</strong>
          <small>клиентов не возвращались</small>
          <div className="inactive-preview">
            {inactive.slice(0, 5).map((x) => (
              <Link key={x.id} href={`/customers/${x.id}`}>
                <span>
                  {x.firstName} {x.lastName}
                </span>
                <small>
                  {x.lastVisitAt
                    ? new Date(x.lastVisitAt).toLocaleDateString("ru-RU")
                    : "Ещё не посещал"}{" "}
                  · {x.phone}
                </small>
              </Link>
            ))}
            {!inactive.length && <p>В этом сегменте сейчас нет клиентов.</p>}
          </div>
          {inactive.length > 5 && (
            <Link className="inactive-more" href={`/customers`}>
              Показать всех в CRM
            </Link>
          )}
        </section>
      </div>
      </div></details>
    </SectionShell>
  );
}
