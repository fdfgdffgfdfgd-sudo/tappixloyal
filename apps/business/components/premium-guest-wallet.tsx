"use client";
import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  Award,
  Check,
  ChevronRight,
  Crown,
  Gift,
  History,
  LockKeyhole,
  LogOut,
  MessageCircle,
  Nfc,
  PartyPopper,
  Share2,
  ShieldCheck,
  Sparkles,
  Star,
  Trophy,
  WalletCards,
} from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
const base = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";
type Profile = {
  id: string;
  firstName: string;
  lastName: string;
  phone: string;
  points: number;
  visits: number;
  level: string;
  company: string;
  companySlug: string;
  logoUrl: string;
  portal: {
    primaryColor?: string;
    secondaryColor?: string;
    cardStyle?: string;
    themeMode?: string;
    loyaltyMode?: string;
    stampsTarget?: number;
    stampReward?: string;
    discountStart?: number;
    discountStep?: number;
    discountMax?: number;
    visitsPerStep?: number;
    promotionTitle?: string;
    promotionText?: string;
    promotionsEnabled?: boolean;
    referralBonus?: number;
  };
};
type Entry = {
  operation: string;
  amount: number;
  balanceAfter: number;
  description: string;
  createdAt: string;
};
type Wallet = {
  level: {
    current: string;
    next: string;
    progress: number;
    remaining: number;
    nextMin: number;
  };
  monthly: { visits: number; earned: number; spent: number; savings: number };
  bonusValue: number;
  bonusExpiry: { date?: string; amount: number };
  achievements: {
    code: string;
    title: string;
    description: string;
    unlocked: boolean;
  }[];
  nextReward: { title: string; remaining: number; target: number };
  referralCode: string;
  referralUrl: string;
  walletPassStatus: string;
  wheel: { canSpin: boolean };
};
const levels: Record<string, { icon: typeof Award; benefit: string }> = {
  Bronze: { icon: Award, benefit: "Бонусы с каждого визита" },
  Silver: { icon: Star, benefit: "Персональные предложения" },
  Gold: { icon: Trophy, benefit: "Повышенные бонусы" },
  Platinum: { icon: Sparkles, benefit: "Приоритетные подарки" },
  Diamond: { icon: Crown, benefit: "Максимум привилегий" },
};
function tokenHeaders(token: string) {
  return { Authorization: `Bearer ${token}` };
}
export function PremiumGuestWallet() {
  const [profile, setProfile] = useState<Profile | null>(null),
    [wallet, setWallet] = useState<Wallet | null>(null),
    [history, setHistory] = useState<Entry[]>([]),
    [token, setToken] = useState(""),
    [mode, setMode] = useState<"whatsapp" | "code" | "pin">("whatsapp"),
    [identity, setIdentity] = useState({ company: "dentline", phone: "" }),
    [devCode, setDevCode] = useState(""),
    [message, setMessage] = useState(""),
    [initializing, setInitializing] = useState(true),
    [historyFilter, setHistoryFilter] = useState<"all" | "credit" | "debit">("all"),
    [spinning, setSpinning] = useState(false),
    [prize, setPrize] = useState("");
  async function load(t: string) {
    const h = { headers: tokenHeaders(t) };
    const [p, w, a] = await Promise.all([
      fetch(`${base}/customer/me`, h).then((r) => r.json()),
      fetch(`${base}/customer/wallet`, h).then((r) => r.json()),
      fetch(`${base}/customer/history`, h).then((r) => r.json()),
    ]);
    if (!p.success) throw new Error(p.error.message);
    setProfile(p.data);
    setWallet(w.data);
    setHistory(a.data || []);
  }
  useEffect(() => {
    const saved = localStorage.getItem("customer_access") || "";
    setToken(saved);
    if (saved)
      load(saved).catch(() => {
        localStorage.removeItem("customer_access");
        setToken("");
      }).finally(() => setInitializing(false));
    else setInitializing(false);
  }, []);
  function accept(data: { accessToken: string; refreshToken: string }) {
    localStorage.setItem("customer_access", data.accessToken);
    localStorage.setItem("customer_refresh", data.refreshToken);
    setToken(data.accessToken);
    void load(data.accessToken);
  }
  async function requestOtp(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setMessage("");
    const value = Object.fromEntries(new FormData(e.currentTarget)) as {
      company: string;
      phone: string;
    };
    const r = await fetch(`${base}/customer/otp/request`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(value),
    }).then((x) => x.json());
    if (!r.success) {
      setMessage(r.error.message);
      return;
    }
    setIdentity(value);
    setDevCode(r.data.devCode || "");
    setMode("code");
  }
  async function verify(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const code = String(new FormData(e.currentTarget).get("code"));
    const r = await fetch(`${base}/customer/otp/verify`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ...identity, code }),
    }).then((x) => x.json());
    if (r.success) accept(r.data);
    else setMessage(r.error.message);
  }
  async function pinLogin(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const r = await fetch(`${base}/customer/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(Object.fromEntries(new FormData(e.currentTarget))),
    }).then((x) => x.json());
    if (r.success) accept(r.data);
    else setMessage(r.error.message);
  }
  async function spin() {
    if (!token || spinning) return;
    setSpinning(true);
    setPrize("");
    setTimeout(async () => {
      const r = await fetch(`${base}/customer/wheel/spin`, {
        method: "POST",
        headers: tokenHeaders(token),
      }).then((x) => x.json());
      setSpinning(false);
      if (r.success) {
        setPrize(r.data.label);
        void load(token);
      } else setMessage(r.error.message);
    }, 1700);
  }
  const grouped = useMemo(
    () =>
      history.filter((entry) => historyFilter === "all" || entry.operation === historyFilter).reduce<Record<string, Entry[]>>((a, x) => {
        const d = new Date(x.createdAt).toLocaleDateString("ru-RU", {
          day: "numeric",
          month: "long",
        });
        (a[d] ??= []).push(x);
        return a;
      }, {}),
    [history, historyFilter],
  );
  if (initializing)
    return <main className="wallet-loading" aria-busy="true" aria-label="Загрузка бонусной карты"><div/><span/><span/><span/></main>;
  if (!token || !profile || !wallet)
    return (
      <main className="premium-auth">
        <div className="premium-auth-brand">
          <span>
            <WalletCards />
          </span>
          <strong>Tappix Wallet</strong>
        </div>
        {mode === "whatsapp" ? (
          <form onSubmit={requestOtp}>
            <span className="auth-orb">
              <MessageCircle />
            </span>
            <small>ЦИФРОВАЯ КАРТА</small>
            <h1>Ваши бонусы всегда рядом</h1>
            <p>Получите одноразовый код в WhatsApp — никаких паролей.</p>
            {message && <div role="alert">{message}</div>}
            <input name="company" type="hidden" value={identity.company} readOnly />
            <div className="auth-company">Карта компании <strong>{identity.company}</strong></div>
            <label>
              Номер WhatsApp
              <input
                name="phone"
                type="tel"
                placeholder="+7 700 000 00 00"
                required
              />
            </label>
            <button>
              Получить код
              <ChevronRight />
            </button>
            <button
              type="button"
              className="auth-link"
              onClick={() => setMode("pin")}
            >
              Войти по резервному коду
            </button>
          </form>
        ) : mode === "code" ? (
          <form onSubmit={verify}>
            <span className="auth-orb">
              <ShieldCheck />
            </span>
            <small>БЕЗОПАСНЫЙ ВХОД</small>
            <h1>Проверьте WhatsApp</h1>
            <p>Код отправлен на {identity.phone} и действует 5 минут.</p>
            {devCode && (
              <div className="premium-dev">
                Тестовый код <b>{devCode}</b>
              </div>
            )}
            {message && <div role="alert">{message}</div>}
            <label>
              6-значный код
              <input
                name="code"
                inputMode="numeric"
                pattern="[0-9]{6}"
                maxLength={6}
                autoFocus
                required
              />
            </label>
            <button>
              Открыть карту
              <ChevronRight />
            </button>
            <button
              type="button"
              className="auth-link"
              onClick={() => setMode("whatsapp")}
            >
              Изменить номер
            </button>
          </form>
        ) : (
          <form onSubmit={pinLogin}>
            <span className="auth-orb">
              <LockKeyhole />
            </span>
            <small>РЕЗЕРВНЫЙ ВХОД</small>
            <h1>Короткий код</h1>
            {message && <div role="alert">{message}</div>}
            <input name="company" type="hidden" value={identity.company} readOnly />
            <div className="auth-company">Карта компании <strong>{identity.company}</strong></div>
            <label>
              Телефон
              <input name="phone" type="tel" required />
            </label>
            <label>
              Код
              <input name="pin" inputMode="numeric" type="password" required />
            </label>
            <button>
              Открыть карту
              <ChevronRight />
            </button>
            <button
              type="button"
              className="auth-link"
              onClick={() => setMode("whatsapp")}
            >
              Войти через WhatsApp
            </button>
          </form>
        )}
      </main>
    );
  const primary = profile.portal?.primaryColor || "#7062ff",
    secondary = profile.portal?.secondaryColor || "#17172c",
    cardNo = `${profile.id.replaceAll("-", "").slice(0, 16)}`
      .match(/.{1,4}/g)
      ?.join(" "),
    LevelIcon = (levels[wallet.level.current] || levels.Bronze).icon,
    stampTarget = Math.max(2, profile.portal?.stampsTarget || 6),
    stamps = profile.visits % stampTarget,
    visitsPerStep = Math.max(1, profile.portal?.visitsPerStep || 3),
    discountStart = Math.max(0, profile.portal?.discountStart || 3),
    discountStep = Math.max(1, profile.portal?.discountStep || 2),
    discountMax = Math.max(discountStart, profile.portal?.discountMax || 15),
    currentDiscount = Math.min(discountMax, discountStart + Math.floor(profile.visits / visitsPerStep) * discountStep),
    visitsToDiscount = currentDiscount >= discountMax ? 0 : visitsPerStep - (profile.visits % visitsPerStep),
    monthLabel = new Intl.DateTimeFormat("ru-RU", { month: "long" }).format(new Date()).toLocaleUpperCase("ru-RU");
  function signOut(){localStorage.removeItem("customer_access");localStorage.removeItem("customer_refresh");setToken("");setProfile(null);setWallet(null)}
  return (
    <main
      className={`premium-wallet theme-${profile.portal?.themeMode || "auto"}`}
      style={
        {
          "--wallet-accent": primary,
          "--wallet-secondary": secondary,
        } as React.CSSProperties
      }
    >
      <header>
        <div>
          <span>{profile.company.slice(0, 1)}</span>
          <strong>{profile.company}</strong>
        </div>
        <nav aria-label="Действия с картой"><button aria-label="Поделиться бонусной картой" onClick={() => navigator.share?.({title: `Карта ${profile.company}`,url: location.href})}><Share2 /></button><button aria-label="Выйти из гостевого кабинета" onClick={signOut}><LogOut/></button></nav>
      </header>
      <section className="wallet-stage" id="wallet-card">
        <div className="loyalty-card">
          <i className="card-aurora" />
          <div className="card-top">
            <div className="card-company">
              <span>{profile.company.slice(0, 1)}</span>
              <strong>{profile.company}</strong>
            </div>
            <span className="nfc-chip">
              <Nfc />
              NFC
            </span>
          </div>
          <div className="card-balance">
            <small>ДОСТУПНО</small>
            <strong>{profile.points.toLocaleString("ru-RU")}</strong>
            <span>бонусов · ≈ {Math.round(wallet.bonusValue || profile.points).toLocaleString("ru-RU")} ₸</span>
          </div>
          <div className="card-owner">
            <div>
              <small>
                {profile.firstName} {profile.lastName}
              </small>
              <span>
                <LevelIcon />
                {wallet.level.current}
              </span>
              <code>{cardNo}</code>
            </div>
            <div className="card-qr">
              <QRCodeSVG
                value={`tappix:customer:${profile.companySlug}:${profile.id}`}
                size={86}
                level="M"
                bgColor="#ffffff"
                fgColor="#12121a"
              />
              <small>Показать на кассе</small>
            </div>
          </div>
        </div>
        <p>
          <Sparkles />
          Ваша цифровая карта готова к использованию
        </p>
      </section>
      <div className="wallet-content">
        <header className="wallet-content-head"><div><small>ДОБРО ПОЖАЛОВАТЬ</small><h1>{profile.firstName}, всё важное здесь</h1></div><a href="#wallet-history">История</a></header>
        <section className="wallet-status-row"><article><Gift/><span><small>СЛЕДУЮЩАЯ НАГРАДА</small><strong>{wallet.nextReward.remaining>0?`Ещё ${wallet.nextReward.remaining} посещ.`:"Уже доступна"}</strong></span></article><article><History/><span><small>СРОК БОНУСОВ</small><strong>{wallet.bonusExpiry?.date?`${wallet.bonusExpiry.amount} сгорят ${new Date(wallet.bonusExpiry.date).toLocaleDateString("ru-RU",{day:"numeric",month:"short"})}`:"Не сгорают"}</strong></span></article></section>
        {profile.portal?.loyaltyMode === "stamps" && (
          <section className="stamp-card">
            <header>
              <div>
                <small>КАРТА ПОСЕЩЕНИЙ</small>
                <h2>{profile.portal.stampReward || "Подарок"}</h2>
              </div>
              <b>
                {stamps}/{stampTarget}
              </b>
            </header>
            <div>
              {Array.from({ length: stampTarget }, (_, index) => (
                <span className={index < stamps ? "filled" : ""} key={index}>
                  {index < stamps ? <Check /> : index + 1}
                </span>
              ))}
            </div>
            <p>
              {stampTarget - stamps === 1
                ? "Ещё один визит до награды"
                : `Осталось ${stampTarget - stamps} визита до награды`}
            </p>
          </section>
        )}
        {profile.portal?.loyaltyMode === "discount" && (
          <section className="discount-card">
            <header><div><small>ВАША ПЕРСОНАЛЬНАЯ СКИДКА</small><h2>{currentDiscount}%</h2></div><b>до {discountMax}%</b></header>
            <div className="level-track"><i style={{width:`${Math.min(100,currentDiscount/discountMax*100)}%`}}/></div>
            <p>{visitsToDiscount === 0 ? "Максимальная скидка уже доступна" : `Ещё ${visitsToDiscount} виз. — скидка вырастет до ${Math.min(discountMax,currentDiscount+discountStep)}%`}</p>
          </section>
        )}
        <section className="level-journey">
          <header>
            <div>
              <small>ТЕКУЩИЙ УРОВЕНЬ</small>
              <h2>
                <LevelIcon />
                {wallet.level.current}
              </h2>
            </div>
            <b>{wallet.level.next}</b>
          </header>
          <div className="level-track">
            <i style={{ width: `${wallet.level.progress}%` }} />
          </div>
          <div>
            <span>
              {profile.points} / {wallet.level.nextMin} бонусов
            </span>
            <strong>
              До {wallet.level.next} осталось {wallet.level.remaining}
            </strong>
          </div>
          <p>{(levels[wallet.level.current] || levels.Bronze).benefit}</p>
        </section>
        <section className="next-goal" id="next-reward">
          <span>
            <Gift />
          </span>
          <div>
            <small>СЛЕДУЮЩАЯ НАГРАДА</small>
            <h2>{wallet.nextReward.title}</h2>
            <p>
              {wallet.nextReward.remaining > 0
                ? `Осталось ${wallet.nextReward.remaining} посещ.`
                : "Награда уже доступна"}
            </p>
          </div>
          <b>
            {Math.min(
              100,
              ((profile.visits % wallet.nextReward.target) /
                wallet.nextReward.target) *
                100,
            ).toFixed(0)}
            %
          </b>
        </section>
        {profile.portal?.promotionsEnabled !== false && (
          <section className="personal-offer">
            <small>ТОЛЬКО ДЛЯ ВАС</small>
            <h2>
              {profile.portal?.promotionTitle ||
                "Двойные бонусы на этой неделе"}
            </h2>
            <p>
              {profile.portal?.promotionText ||
                "Запишитесь до воскресенья и получите больше бонусов за посещение."}
            </p>
            <button>
              Узнать подробнее
              <ChevronRight />
            </button>
          </section>
        )}
        <details className="wallet-more">
          <summary><span><Gift/>Больше возможностей</span><small>Статистика, колесо и достижения</small><ChevronRight/></summary>
          <div className="wallet-more-content">
        <section className="month-value">
          <header>
            <small>{monthLabel}</small>
            <h2>Ваша польза за месяц</h2>
          </header>
          <div>
            <article>
              <b>{wallet.monthly.visits}</b>
              <span>Посещений</span>
            </article>
            <article>
              <b>+{wallet.monthly.earned}</b>
              <span>Получено</span>
            </article>
            <article>
              <b>−{wallet.monthly.spent}</b>
              <span>Использовано</span>
            </article>
            <article>
              <b>{wallet.monthly.savings.toLocaleString("ru-RU")} ₸</b>
              <span>Экономия</span>
            </article>
          </div>
        </section>
        <section className="lucky-card">
          <div className={`lucky-wheel ${spinning ? "is-spinning" : ""}`}>
            <span>
              <Gift />
            </span>
            <i />
            <i />
            <i />
          </div>
          <div>
            <small>РАЗ В НЕДЕЛЮ</small>
            <h2>Счастливое колесо</h2>
            <p>
              {prize
                ? `Ваш приз: ${prize}`
                : wallet.wheel.canSpin
                  ? "Испытайте удачу и заберите подарок"
                  : "Новая попытка через неделю"}
            </p>
            <button disabled={!wallet.wheel.canSpin || spinning} onClick={spin}>
              {spinning ? "Крутим…" : "Крутить колесо"}
            </button>
          </div>
        </section>
        <section className="achievements">
          <header>
            <small>ВАША КОЛЛЕКЦИЯ</small>
            <h2>Достижения</h2>
          </header>
          <div>
            {wallet.achievements.map((x, i) => (
              <article
                className={x.unlocked ? "unlocked" : "locked"}
                key={x.code}
              >
                <span>
                  {i === 4 ? (
                    <Crown />
                  ) : i === 3 ? (
                    <Star />
                  ) : i === 2 ? (
                    <Trophy />
                  ) : (
                    <Award />
                  )}
                </span>
                <strong>{x.title}</strong>
                <small>{x.description}</small>
              </article>
            ))}
          </div>
        </section>
          </div>
        </details>
        <section className="activity-timeline" id="wallet-history">
          <header>
            <div><small>ПОСЛЕДНИЕ СОБЫТИЯ</small><h2>Активность</h2></div>
            <div className="history-filters" aria-label="Фильтр истории">{([['all','Все'],['credit','Начислено'],['debit','Списано']] as const).map(([value,label])=><button className={historyFilter===value?'current':''} aria-pressed={historyFilter===value} onClick={()=>setHistoryFilter(value)} key={value}>{label}</button>)}</div>
          </header>
          {Object.entries(grouped)
            .slice(0, 4)
            .map(([date, items]) => (
              <div className="timeline-day" key={date}>
                <h3>{date}</h3>
                {items.map((x, i) => (
                  <article key={i}>
                    <i
                      className={x.operation === "credit" ? "credit" : "debit"}
                    >
                      {x.operation === "credit" ? <Sparkles /> : <History />}
                    </i>
                    <div>
                      <strong>{x.description}</strong>
                      <small>Баланс после операции: {x.balanceAfter}</small>
                    </div>
                    <b>
                      {x.operation === "credit" ? "+" : "−"}
                      {x.amount}
                    </b>
                  </article>
                ))}
              </div>
            ))}
          {!Object.keys(grouped).length && <p className="wallet-empty">Операций этого типа пока нет</p>}
        </section>
        <button
          className="refer-card"
          onClick={() =>
            navigator.share?.({
              title: `Присоединяйтесь к ${profile.company}`,
              url: wallet.referralUrl,
            })
          }
        >
          <span>
            <Share2 />
          </span>
          <div>
            <small>ПРИГЛАСИТЬ ДРУГА</small>
            <strong>
              Получите +{profile.portal?.referralBonus || 100} бонусов
            </strong>
            <code>{wallet.referralCode}</code>
          </div>
          <ChevronRight />
        </button>
        <section className="wallet-future">
          <WalletCards />
          <div>
            <strong>Добавить в Wallet</strong>
            <small>Apple Wallet и Google Wallet — скоро</small>
          </div>
          <span>В разработке</span>
        </section>
        <footer>
          Защищено и работает на <strong>Tappix</strong>
        </footer>
      </div>
      {prize && (
        <div className="prize-overlay" onClick={() => setPrize("")}>
          <div role="dialog" aria-modal="true" aria-labelledby="prize-title" onClick={event => event.stopPropagation()}>
            <PartyPopper />
            <small>ПОЗДРАВЛЯЕМ!</small>
            <h2 id="prize-title">{prize}</h2>
            <p>Приз уже добавлен в ваш кабинет.</p>
            <button onClick={() => setPrize("")}>Забрать подарок</button>
          </div>
        </div>
      )}
      <nav className="wallet-tabbar" aria-label="Навигация по карте"><a href="#wallet-card"><WalletCards/><span>Карта</span></a><a href="#next-reward"><Gift/><span>Награда</span></a><a href="#wallet-history"><History/><span>История</span></a></nav>
    </main>
  );
}
