"use client";
import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import {
  Blocks,
  Building2,
  Check,
  ChevronRight,
  CreditCard,
  Eye,
  ImageUp,
  LockKeyhole,
  Palette,
  Percent,
  Plug,
  Smartphone,
  Stamp,
  Store,
  Users,
} from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
type Company = {
  name: string;
  phone: string;
  email: string;
  address: string;
  timezone: string;
  language: string;
};
type Module = { code: string; name: string; core: boolean; enabled: boolean };
type Subscription = {
  plan: string;
  status: string;
  amount: number;
  periodEndsAt?: string;
};
type GuestSettings = {
  welcomeTitle: string;
  welcomeText: string;
  primaryColor: string;
  secondaryColor: string;
  cardStyle: string;
  themeMode: string;
  backgroundUrl: string;
  logoUrl: string;
  requireEmail: boolean;
  requireCity: boolean;
  showGender: boolean;
  promotionsEnabled: boolean;
  promotionTitle: string;
  promotionText: string;
  referralBonus: number;
  loyaltyMode: string;
  stampsTarget: number;
  stampReward: string;
  discountStart: number;
  discountStep: number;
  discountMax: number;
  visitsPerStep: number;
  whatsapp: string;
  instagram: string;
  website: string;
  mapUrl: string;
};
const guestDefaults: GuestSettings = {
  welcomeTitle: "",
  welcomeText: "",
  primaryColor: "#5B4AE8",
  secondaryColor: "#17172c",
  cardStyle: "aurora",
  themeMode: "auto",
  backgroundUrl: "",
  logoUrl: "",
  requireEmail: false,
  requireCity: false,
  showGender: true,
  promotionsEnabled: true,
  promotionTitle: "Специальное предложение",
  promotionText: "",
  referralBonus: 100,
  loyaltyMode: "points",
  stampsTarget: 6,
  stampReward: "Подарок",
  discountStart: 3,
  discountStep: 2,
  discountMax: 15,
  visitsPerStep: 3,
  whatsapp: "",
  instagram: "",
  website: "",
  mapUrl: "",
};
const presets = [
  { id: "aurora", name: "Aurora", a: "#5B4AE8", b: "#17172c" },
  { id: "berry", name: "Berry", a: "#8B1E4F", b: "#24131d" },
  { id: "emerald", name: "Emerald", a: "#087F5B", b: "#10231d" },
  { id: "sunset", name: "Sunset", a: "#E8590C", b: "#351a12" },
];
const extensionMeta: Record<
  string,
  { label: string; description: string; href: string }
> = {
  email: {
    label: "Email-рассылки",
    description: "Персональные письма и сегменты",
    href: "/campaigns",
  },
  sms: {
    label: "SMS",
    description: "Транзакционные сообщения",
    href: "/notifications",
  },
  telegram: {
    label: "Telegram",
    description: "Бот и уведомления",
    href: "/integrations",
  },
  api: {
    label: "API",
    description: "Подключение внешних систем",
    href: "/api-keys",
  },
  booking: {
    label: "Онлайн-запись",
    description: "Публичная форма записи",
    href: "/bookings",
  },
  website: {
    label: "Mini Site",
    description: "Страница компании",
    href: "/website",
  },
  integrations: {
    label: "Интеграции",
    description: "CRM и webhooks",
    href: "/integrations",
  },
  automation: {
    label: "Автоматизация",
    description: "Автоматические сценарии",
    href: "/loyalty",
  },
  ai: { label: "AI", description: "Экспериментальные возможности", href: "#" },
};
export function MVPSettingsPage() {
  const [value, setValue] = useState<Company>({
      name: "",
      phone: "",
      email: "",
      address: "",
      timezone: "Asia/Almaty",
      language: "ru",
    }),
    [guest, setGuest] = useState<GuestSettings>(guestDefaults),
    [modules, setModules] = useState<Module[]>([]),
    [subscription, setSubscription] = useState<Subscription | null>(null),
    [message, setMessage] = useState(""),
    [saving, setSaving] = useState(false);
  useEffect(() => {
    Promise.all([
      api<Company>("/settings/company"),
      api<Module[]>("/modules"),
      api<Subscription>("/subscription"),
      api<Partial<GuestSettings>>("/settings/guest-portal"),
    ])
      .then(([c, m, s, g]) => {
        setValue(c);
        setModules(m);
        setSubscription(s);
        setGuest({ ...guestDefaults, ...g });
      })
      .catch((e) => setMessage(e.message));
  }, []);
  async function save(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      setValue(
        await api<Company>("/settings/company", {
          method: "PATCH",
          body: JSON.stringify(value),
        }),
      );
      setMessage("Настройки компании сохранены");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Ошибка сохранения");
    } finally {
      setSaving(false);
    }
  }
  async function saveGuest(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      setGuest(
        await api<GuestSettings>("/settings/guest-portal", {
          method: "PATCH",
          body: JSON.stringify(guest),
        }),
      );
      setMessage("Guest Portal опубликован");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Ошибка сохранения");
    } finally {
      setSaving(false);
    }
  }
  async function uploadBrand(file: File, kind: "logo" | "background") {
    setSaving(true);
    try {
      const body = new FormData();
      body.set("file", file);
      body.set("kind", kind === "logo" ? "logo" : "asset");
      const uploaded = await api<{ url: string }>("/upload", {
        method: "POST",
        body,
      });
      setGuest((current) => ({
        ...current,
        [kind === "logo" ? "logoUrl" : "backgroundUrl"]: uploaded.url,
      }));
      setMessage(kind === "logo" ? "Логотип загружен" : "Фон загружен");
    } catch (e) {
      setMessage(
        e instanceof Error ? e.message : "Не удалось загрузить изображение",
      );
    } finally {
      setSaving(false);
    }
  }
  const extensions = modules.filter((m) => !m.core && extensionMeta[m.code]);
  const active = extensions.filter((m) => m.enabled);
  return (
    <SectionShell
      active="/settings"
      title="Настройки"
      subtitle="Компания, команда и дополнительные возможности"
    >
      {message && (
        <div className="mvp-settings-notice" role="status">
          <Check />
          {message}
        </div>
      )}
      <div className="mvp-settings-layout">
        <div className="mvp-settings-main">
          <section className="mvp-settings-card">
            <div className="mvp-settings-title">
              <span>
                <Building2 />
              </span>
              <div>
                <h2>Компания</h2>
                <p>Основные данные текущего рабочего пространства.</p>
              </div>
            </div>
            <form onSubmit={save}>
              <div className="mvp-settings-grid">
                <label>
                  Название
                  <input
                    value={value.name}
                    onChange={(e) =>
                      setValue({ ...value, name: e.target.value })
                    }
                    required
                  />
                </label>
                <label>
                  Телефон
                  <input
                    value={value.phone}
                    onChange={(e) =>
                      setValue({ ...value, phone: e.target.value })
                    }
                  />
                </label>
                <label>
                  Email
                  <input
                    type="email"
                    value={value.email}
                    onChange={(e) =>
                      setValue({ ...value, email: e.target.value })
                    }
                  />
                </label>
                <label>
                  Адрес
                  <input
                    value={value.address}
                    onChange={(e) =>
                      setValue({ ...value, address: e.target.value })
                    }
                  />
                </label>
              </div>
              <button disabled={saving}>
                {saving ? "Сохраняем…" : "Сохранить"}
              </button>
            </form>
          </section>
          <section className="mvp-settings-card">
            <div className="mvp-settings-title">
              <span>
                <Store />
              </span>
              <div>
                <h2>Структура бизнеса</h2>
                <p>
                  Настройте только то, что необходимо команде для ежедневной
                  работы.
                </p>
              </div>
            </div>
            <div className="mvp-settings-links">
              <Link href="/branches">
                <Store />
                <span>
                  <strong>Филиалы</strong>
                  <small>Адреса и точки обслуживания</small>
                </span>
                <ChevronRight />
              </Link>
              <Link href="/employees">
                <Users />
                <span>
                  <strong>Команда и доступы</strong>
                  <small>Сотрудники, роли и приглашения</small>
                </span>
                <ChevronRight />
              </Link>
              <Link href="/subscription">
                <CreditCard />
                <span>
                  <strong>Тариф</strong>
                  <small>
                    {subscription?.plan || "Starter"} ·{" "}
                    {subscription?.status || "trial"}
                  </small>
                </span>
                <ChevronRight />
              </Link>
            </div>
          </section>
          <section className="mvp-settings-card design-studio">
            <div className="mvp-settings-title">
              <span>
                <Palette />
              </span>
              <div>
                <h2>Студия дизайна карты</h2>
                <p>
                  Выберите стиль, цвета и механику — изменения появятся у гостей
                  после сохранения Guest Portal.
                </p>
              </div>
            </div>
            <fieldset className="design-presets">
              <legend>Готовые стили</legend>
              <div>
                {presets.map((x) => (
                  <button
                    type="button"
                    className={guest.cardStyle === x.id ? "selected" : ""}
                    onClick={() =>
                      setGuest({
                        ...guest,
                        cardStyle: x.id,
                        primaryColor: x.a,
                        secondaryColor: x.b,
                      })
                    }
                    key={x.id}
                  >
                    <i
                      style={{
                        background: `linear-gradient(135deg,${x.a},${x.b})`,
                      }}
                    />
                    <span>{x.name}</span>
                    {guest.cardStyle === x.id && <Check />}
                  </button>
                ))}
              </div>
            </fieldset>
            <div className="brand-upload-grid">
              <label className="brand-upload">
                <ImageUp />
                <span>
                  <strong>Загрузить логотип</strong>
                  <small>
                    {guest.logoUrl
                      ? "Загружен — можно заменить"
                      : "PNG, JPG или WebP"}
                  </small>
                </span>
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  onChange={(e) =>
                    e.target.files?.[0] &&
                    void uploadBrand(e.target.files[0], "logo")
                  }
                />
              </label>
              <label className="brand-upload">
                <ImageUp />
                <span>
                  <strong>Загрузить фон</strong>
                  <small>
                    {guest.backgroundUrl
                      ? "Загружен — можно заменить"
                      : "Вертикальное изображение"}
                  </small>
                </span>
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  onChange={(e) =>
                    e.target.files?.[0] &&
                    void uploadBrand(e.target.files[0], "background")
                  }
                />
              </label>
            </div>
            <fieldset className="loyalty-mode-picker">
              <legend>Механика программы</legend>
              <div>
                <label
                  className={guest.loyaltyMode === "points" ? "selected" : ""}
                >
                  <input
                    type="radio"
                    checked={guest.loyaltyMode === "points"}
                    onChange={() =>
                      setGuest({ ...guest, loyaltyMode: "points" })
                    }
                  />
                  <Palette />
                  <span>
                    <strong>Бонусы</strong>
                    <small>Баланс, начисление и списание</small>
                  </span>
                </label>
                <label
                  className={guest.loyaltyMode === "stamps" ? "selected" : ""}
                >
                  <input
                    type="radio"
                    checked={guest.loyaltyMode === "stamps"}
                    onChange={() =>
                      setGuest({ ...guest, loyaltyMode: "stamps" })
                    }
                  />
                  <Stamp />
                  <span>
                    <strong>Штампы за визиты</strong>
                    <small>Один приход — один штамп</small>
                  </span>
                </label>
                <label className={guest.loyaltyMode === "discount" ? "selected" : ""}>
                  <input type="radio" checked={guest.loyaltyMode === "discount"} onChange={() => setGuest({ ...guest, loyaltyMode: "discount" })}/>
                  <Percent />
                  <span><strong>Растущая скидка</strong><small>Чем чаще приходит, тем выше процент</small></span>
                </label>
              </div>
              {guest.loyaltyMode === "stamps" && (
                <div className="stamp-settings">
                  <label>
                    Штампов до награды
                    <input
                      type="number"
                      min="2"
                      max="20"
                      value={guest.stampsTarget}
                      onChange={(e) =>
                        setGuest({
                          ...guest,
                          stampsTarget: Number(e.target.value),
                        })
                      }
                    />
                  </label>
                  <label>
                    Награда
                    <input
                      value={guest.stampReward}
                      onChange={(e) =>
                        setGuest({ ...guest, stampReward: e.target.value })
                      }
                      placeholder="Бесплатный кофе"
                    />
                  </label>
                </div>
              )}
              {guest.loyaltyMode === "discount" && (
                <div className="stamp-settings discount-settings">
                  <label>Стартовая скидка, %<input type="number" min="0" max="50" value={guest.discountStart} onChange={e=>setGuest({...guest,discountStart:Number(e.target.value)})}/></label>
                  <label>Рост скидки, %<input type="number" min="1" max="20" value={guest.discountStep} onChange={e=>setGuest({...guest,discountStep:Number(e.target.value)})}/></label>
                  <label>Каждые N визитов<input type="number" min="1" max="50" value={guest.visitsPerStep} onChange={e=>setGuest({...guest,visitsPerStep:Number(e.target.value)})}/></label>
                  <label>Максимальная скидка, %<input type="number" min="1" max="70" value={guest.discountMax} onChange={e=>setGuest({...guest,discountMax:Number(e.target.value)})}/></label>
                </div>
              )}
            </fieldset>
          </section>
          <section className="mvp-settings-card guest-settings-card">
            <div className="mvp-settings-title">
              <span>
                <Smartphone />
              </span>
              <div>
                <h2>Личный кабинет гостя</h2>
                <p>Бренд, регистрация, акции и контакты после касания NFC.</p>
              </div>
            </div>
            <form onSubmit={saveGuest}>
              <div
                className="guest-settings-preview"
                style={
                  {
                    "--preview-color": guest.primaryColor,
                  } as React.CSSProperties
                }
              >
                <div>
                  <Palette />
                  <span>
                    <small>Предпросмотр карты</small>
                    <strong>{value.name || "Ваша компания"}</strong>
                  </span>
                </div>
                <b>
                  420 <small>бонусов</small>
                </b>
              </div>
              <div className="mvp-settings-grid">
                <label>
                  Заголовок приветствия
                  <input
                    value={guest.welcomeTitle}
                    onChange={(e) =>
                      setGuest({ ...guest, welcomeTitle: e.target.value })
                    }
                    placeholder={`Добро пожаловать в ${value.name || "компанию"}!`}
                  />
                </label>
                <label>
                  Фирменный цвет
                  <span className="color-input">
                    <input
                      type="color"
                      value={guest.primaryColor}
                      onChange={(e) =>
                        setGuest({ ...guest, primaryColor: e.target.value })
                      }
                    />
                    <input
                      value={guest.primaryColor}
                      onChange={(e) =>
                        setGuest({ ...guest, primaryColor: e.target.value })
                      }
                    />
                  </span>
                </label>
                <label className="wide">
                  Приветственный текст
                  <textarea
                    rows={3}
                    value={guest.welcomeText}
                    onChange={(e) =>
                      setGuest({ ...guest, welcomeText: e.target.value })
                    }
                    placeholder="Копите бонусы за каждое посещение…"
                  />
                </label>
                <label>
                  URL логотипа
                  <input
                    type="url"
                    value={guest.logoUrl}
                    onChange={(e) =>
                      setGuest({ ...guest, logoUrl: e.target.value })
                    }
                  />
                </label>
                <label>
                  URL фоновой картинки
                  <input
                    type="url"
                    value={guest.backgroundUrl}
                    onChange={(e) =>
                      setGuest({ ...guest, backgroundUrl: e.target.value })
                    }
                  />
                </label>
              </div>
              <fieldset className="guest-setting-options">
                <legend>Поля регистрации</legend>
                <label>
                  <input
                    type="checkbox"
                    checked={guest.showGender}
                    onChange={(e) =>
                      setGuest({ ...guest, showGender: e.target.checked })
                    }
                  />
                  Показывать выбор пола
                </label>
                <label>
                  <input
                    type="checkbox"
                    checked={guest.requireEmail}
                    onChange={(e) =>
                      setGuest({ ...guest, requireEmail: e.target.checked })
                    }
                  />
                  Email обязателен
                </label>
                <label>
                  <input
                    type="checkbox"
                    checked={guest.requireCity}
                    onChange={(e) =>
                      setGuest({ ...guest, requireCity: e.target.checked })
                    }
                  />
                  Город обязателен
                </label>
              </fieldset>
              <details className="guest-settings-more">
                <summary>
                  <Eye />
                  Акции, рефералы и контакты
                </summary>
                <div className="mvp-settings-grid">
                  <label className="toggle-wide">
                    <input
                      type="checkbox"
                      checked={guest.promotionsEnabled}
                      onChange={(e) =>
                        setGuest({
                          ...guest,
                          promotionsEnabled: e.target.checked,
                        })
                      }
                    />
                    Показывать блок акций
                  </label>
                  <label>
                    Название акции
                    <input
                      value={guest.promotionTitle}
                      onChange={(e) =>
                        setGuest({ ...guest, promotionTitle: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Бонус за друга
                    <input
                      type="number"
                      min="0"
                      value={guest.referralBonus}
                      onChange={(e) =>
                        setGuest({
                          ...guest,
                          referralBonus: Number(e.target.value),
                        })
                      }
                    />
                  </label>
                  <label className="wide">
                    Описание акции
                    <textarea
                      rows={2}
                      value={guest.promotionText}
                      onChange={(e) =>
                        setGuest({ ...guest, promotionText: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    WhatsApp URL
                    <input
                      type="url"
                      value={guest.whatsapp}
                      onChange={(e) =>
                        setGuest({ ...guest, whatsapp: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Instagram URL
                    <input
                      type="url"
                      value={guest.instagram}
                      onChange={(e) =>
                        setGuest({ ...guest, instagram: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Сайт
                    <input
                      type="url"
                      value={guest.website}
                      onChange={(e) =>
                        setGuest({ ...guest, website: e.target.value })
                      }
                    />
                  </label>
                  <label>
                    Маршрут / карта
                    <input
                      type="url"
                      value={guest.mapUrl}
                      onChange={(e) =>
                        setGuest({ ...guest, mapUrl: e.target.value })
                      }
                    />
                  </label>
                </div>
              </details>
              <button disabled={saving}>
                {saving ? "Публикуем…" : "Сохранить Guest Portal"}
              </button>
            </form>
          </section>
          {active.length > 0 && (
            <section className="mvp-settings-card">
              <div className="mvp-settings-title">
                <span>
                  <Plug />
                </span>
                <div>
                  <h2>Активные расширения</h2>
                  <p>Дополнительные инструменты вашего тарифа.</p>
                </div>
              </div>
              <div className="mvp-extension-grid">
                {active.map((m) => {
                  const meta = extensionMeta[m.code];
                  return (
                    <Link href={meta.href} key={m.code}>
                      <Blocks />
                      <span>
                        <strong>{meta.label}</strong>
                        <small>{meta.description}</small>
                      </span>
                      <ChevronRight />
                    </Link>
                  );
                })}
              </div>
            </section>
          )}
          <details className="mvp-extension-catalog">
            <summary>
              <span>
                <LockKeyhole />
                Дополнительные функции
              </span>
              <small>{extensions.length} модулей</small>
            </summary>
            <div>
              <p>
                Расширения не перегружают интерфейс и появляются только после
                подключения подходящего тарифа.
              </p>
              <div className="mvp-extension-grid">
                {extensions.map((m) => {
                  const meta = extensionMeta[m.code];
                  return (
                    <article key={m.code}>
                      <Blocks />
                      <span>
                        <strong>{meta.label}</strong>
                        <small>{meta.description}</small>
                      </span>
                      <b>{m.enabled ? "Подключено" : "Не подключено"}</b>
                    </article>
                  );
                })}
              </div>
            </div>
          </details>
        </div>
        <aside className="mvp-principle">
          <span>
            <Blocks />
          </span>
          <h2>Простой Tappix</h2>
          <p>
            В основном меню остаются только функции, которые помогают работать с
            лояльностью каждую неделю.
          </p>
          <ul>
            <li>Клиенты и посещения</li>
            <li>Бонусы и награды</li>
            <li>NFC и отзывы</li>
            <li>Понятная аналитика</li>
          </ul>
        </aside>
      </div>
    </SectionShell>
  );
}
