"use client";

import { Check, Copy, Gift, History, QrCode, Trophy, X } from "lucide-react";
import Image from "next/image";
import { QRCodeSVG } from "qrcode.react";
import { useDialogFocusTrap } from "./use-dialog-focus-trap";
type Profile = {
  firstName: string;
  company: string;
  logoUrl: string;
  points: number;
  portal: { primaryColor?: string };
};
type Wallet = {
  customerCode: string;
  loyalty: {
    mode: string;
    progress: number;
    remaining: number;
    target: number;
    rewardTitle: string;
    balanceValue: number;
    balancePoints: number;
  };
  bonusExpiry: { date?: string; amount: number };
  nextReward: { title: string; remaining: number };
};
type Reward = { id: string; name: string; status: string; expiresAt?: string };
type Entry = {
  operation: string;
  amount: number;
  createdAt: string;
  description: string;
};
export function ConsumerWalletHome({
  profile,
  wallet,
  rewards,
  history,
  qrOpen,
  setQrOpen,
  tab,
  setTab,
  signOut,
}: {
  profile: Profile;
  wallet: Wallet;
  rewards: Reward[];
  history: Entry[];
  qrOpen: boolean;
  setQrOpen: (v: boolean) => void;
  tab: "home" | "rewards" | "history";
  setTab: (v: "home" | "rewards" | "history") => void;
  signOut: () => void;
}) {
  const mode = wallet.loyalty.mode,
    visits = wallet.loyalty.progress,
    target = wallet.loyalty.target,
    remaining = wallet.loyalty.remaining;
  const closeQr = () => setQrOpen(false);
  const qrDialog = useDialogFocusTrap<HTMLDivElement>(qrOpen, closeQr);
  return (
    <main
      className="consumer-wallet-shell"
      style={
        {
          "--consumer-brand": profile.portal.primaryColor || "#0082d2",
        } as React.CSSProperties
      }
    >
      <header className="consumer-wallet-header">
        <span className="consumer-business-mark">
          {profile.logoUrl ? (
            <Image src={profile.logoUrl} alt="" width={36} height={36} unoptimized />
          ) : (
            profile.company.slice(0, 1)
          )}
        </span>
        <div>
          <strong>{profile.company}</strong>
          <small>Ваша карта клиента</small>
        </div>
        <button aria-label="Выйти" onClick={signOut}>
          Выйти
        </button>
      </header>
      <div className="consumer-wallet-content">
        {tab === "home" && (
          <>
            <section className="consumer-loyalty-hero">
              <small>{profile.firstName}</small>
              {mode === "stamps" ? (
                <>
                  <p>Ваш прогресс</p>
                  <h1>
                    {visits} из {target} посещений
                  </h1>
                  <div className="consumer-progress-dots">
                    {Array.from({ length: target }, (_, i) => (
                      <i className={i < visits ? "filled" : ""} key={i}>
                        {i < visits ? <Check /> : null}
                      </i>
                    ))}
                  </div>
                  <strong>
                    {remaining === 0
                      ? "Награда доступна"
                      : remaining === 1
                        ? "Остался 1 визит"
                        : `Ещё ${remaining} посещений`}
                  </strong>
                  <span className="consumer-reward-line">
                    <Gift /> {wallet.loyalty.rewardTitle}
                  </span>
                </>
              ) : (
                <>
                  <p>Ваш баланс</p>
                  <h1>
                    {Math.round(wallet.loyalty.balanceValue).toLocaleString(
                      "ru-RU",
                    )}{" "}
                    ₸
                  </h1>
                  <span>
                    {wallet.loyalty.balancePoints.toLocaleString("ru-RU")}{" "}
                    бонусов
                  </span>
                  <strong>
                    {wallet.nextReward.remaining === 0
                      ? "Награда доступна"
                      : `До следующей награды ${wallet.nextReward.remaining}`}
                  </strong>
                </>
              )}
              <button
                className="consumer-qr-cta"
                onClick={() => setQrOpen(true)}
              >
                <QrCode />
                Показать карту на кассе
              </button>
            </section>
            <section className="consumer-next-reward">
              <Gift />
              <div>
                <small>СЛЕДУЮЩАЯ НАГРАДА</small>
                <h2>
                  {mode === "stamps"
                    ? wallet.loyalty.rewardTitle
                    : wallet.nextReward.title}
                </h2>
                <p>
                  {remaining === 0
                    ? "Можно использовать сейчас"
                    : remaining === 1
                      ? "Остался 1 визит"
                      : `Осталось ${remaining} посещений`}
                </p>
              </div>
            </section>
            <section className="consumer-recent">
              <header>
                <h2>Последняя активность</h2>
                <button onClick={() => setTab("history")}>Вся история</button>
              </header>
              {history.slice(0, 3).map((item, i) => (
                <article key={`${item.createdAt}-${i}`}>
                  <span>
                    {item.operation.includes("visit") ? <Check /> : <History />}
                  </span>
                  <div>
                    <strong>{item.description || "Операция по карте"}</strong>
                    <small>
                      {new Date(item.createdAt).toLocaleDateString("ru-RU")}
                    </small>
                  </div>
                  <b>{item.amount > 0 ? `+${item.amount}` : ""}</b>
                </article>
              ))}
              {!history.length && (
                <p>Здесь появятся ваши посещения и бонусы.</p>
              )}
            </section>
          </>
        )}
        {tab === "rewards" && (
          <section className="consumer-page-section">
            <small>НАГРАДЫ</small>
            <h1>Ваши награды</h1>
            {rewards.map((r) => (
              <article className={`consumer-reward-row ${r.status}`} key={r.id}>
                <Gift />
                <div>
                  <strong>{r.name}</strong>
                  <small>
                    {r.status === "available"
                      ? "Доступно"
                      : r.status === "redeemed"
                        ? "Использовано"
                        : r.status === "expired"
                          ? "Срок истёк"
                          : "Ещё в пути"}
                  </small>
                </div>
              </article>
            ))}
            {!rewards.length && (
              <p>Награды появятся здесь по мере ваших посещений.</p>
            )}
          </section>
        )}
        {tab === "history" && (
          <section className="consumer-page-section">
            <small>ИСТОРИЯ</small>
            <h1>Ваши визиты</h1>
            {history.map((item, i) => (
              <article
                className="consumer-history-row"
                key={`${item.createdAt}-${i}`}
              >
                <History />
                <div>
                  <strong>{item.description || "Посещение"}</strong>
                  <small>
                    {new Date(item.createdAt).toLocaleDateString("ru-RU")}
                  </small>
                </div>
                <b>{item.amount > 0 ? `+${item.amount}` : ""}</b>
              </article>
            ))}
            {!history.length && <p>Здесь появятся ваши посещения и бонусы.</p>}
          </section>
        )}
      </div>
      <nav className="consumer-bottom-nav">
        <button
          className={tab === "home" ? "active" : ""}
          onClick={() => setTab("home")}
        >
          <Trophy />
          Карта
        </button>
        <button
          className={tab === "rewards" ? "active" : ""}
          onClick={() => setTab("rewards")}
        >
          <Gift />
          Награды
        </button>
        <button
          className={tab === "history" ? "active" : ""}
          onClick={() => setTab("history")}
        >
          <History />
          История
        </button>
      </nav>
      {qrOpen && (
        <div
          ref={qrDialog}
          className="consumer-cashier-mode"
          role="dialog"
          aria-modal="true"
          aria-labelledby="consumer-cashier-title"
        >
          <button aria-label="Закрыть" onClick={closeQr}>
            <X />
          </button>
          <small>ПОКАЖИТЕ ЭТОТ КОД СОТРУДНИКУ</small>
          <h1 id="consumer-cashier-title">Покажите этот код сотруднику</h1>
          <div>
            <QRCodeSVG
              value={wallet.customerCode}
              size={250}
              includeMargin
              aria-label="QR-код карты клиента"
            />
          </div>
          <strong>
            {wallet.customerCode.replace(/(\d{3})(\d{3})/, "$1 $2")}
          </strong>
          <small>КОД КЛИЕНТА</small>
          <button
            className="consumer-copy-code"
            onClick={() => void navigator.clipboard.writeText(wallet.customerCode)}
          >
            <Copy />
            Скопировать код
          </button>
          <p>Если QR не сканируется, сотрудник может ввести этот код.</p>
        </div>
      )}
    </main>
  );
}
