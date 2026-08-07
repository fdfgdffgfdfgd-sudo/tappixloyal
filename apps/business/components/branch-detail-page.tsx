"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import {
  ArrowLeft,
  Gift,
  Nfc,
  QrCode,
  ScanLine,
  UserRound,
  Users,
} from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";

type BranchDetail = {
  id: string;
  name: string;
  address: string;
  phone: string;
  active: boolean;
  stats: {
    visits30Days: number;
    uniqueCustomers30Days: number;
    points30Days: number;
  };
  employees: {
    id: string;
    firstName: string;
    lastName: string;
    email: string;
    status: string;
    lastLoginAt?: string;
  }[];
  devices: {
    id: string;
    kind: string;
    name: string;
    destination: string;
    active: boolean;
    scans: number;
    lastScannedAt?: string;
  }[];
};

export function BranchDetailPage({ id }: { id: string }) {
  const [data, setData] = useState<BranchDetail | null>(null);
  const [msg, setMsg] = useState("");
  useEffect(() => {
    api<BranchDetail>(`/branches/${id}`)
      .then(setData)
      .catch((e) =>
        setMsg(e instanceof Error ? e.message : "Не удалось загрузить филиал"),
      );
  }, [id]);
  return (
    <SectionShell
      active="/branches"
      title={data?.name || "Филиал"}
      subtitle={
        data
          ? `${data.address}${data.phone ? ` · ${data.phone}` : ""}`
          : "Загрузка данных…"
      }
    >
      <Link className="branch-back" href="/branches">
        <ArrowLeft />
        Все филиалы
      </Link>
      {msg && (
        <div className="portal-message" role="alert">
          {msg}
        </div>
      )}
      {data && (
        <>
          <div
            className="branch-metrics"
            aria-label="Статистика филиала за 30 дней"
          >
            <article>
              <ScanLine />
              <span>Посещений</span>
              <strong>{data.stats.visits30Days}</strong>
              <small>за 30 дней</small>
            </article>
            <article>
              <Users />
              <span>Клиентов</span>
              <strong>{data.stats.uniqueCustomers30Days}</strong>
              <small>уникальных</small>
            </article>
            <article>
              <Gift />
              <span>Бонусов</span>
              <strong>{data.stats.points30Days}</strong>
              <small>начислено</small>
            </article>
          </div>
          <div className="branch-detail-grid">
            <section className="branch-panel">
              <div className="branch-panel-title">
                <div>
                  <h2>Сотрудники</h2>
                  <p>{data.employees.length} привязано к филиалу</p>
                </div>
                <Link href="/employees">Управлять</Link>
              </div>
              {data.employees.map((x) => (
                <article className="branch-row" key={x.id}>
                  <span>
                    <UserRound />
                  </span>
                  <div>
                    <strong>
                      {x.firstName} {x.lastName}
                    </strong>
                    <small>{x.email}</small>
                  </div>
                  <b className={x.status === "active" ? "status" : "tag"}>
                    {x.status === "active" ? "Активен" : "Заблокирован"}
                  </b>
                </article>
              ))}
              {!data.employees.length && (
                <div className="branch-empty">
                  <UserRound />
                  <p>К филиалу пока не привязаны сотрудники.</p>
                  <Link href="/employees">Добавить сотрудника</Link>
                </div>
              )}
            </section>
            <section className="branch-panel">
              <div className="branch-panel-title">
                <div>
                  <h2>NFC и QR</h2>
                  <p>{data.devices.length} устройств</p>
                </div>
                <Link href="/devices">Управлять</Link>
              </div>
              {data.devices.map((x) => (
                <article className="branch-row" key={x.id}>
                  <span>{x.kind === "nfc" ? <Nfc /> : <QrCode />}</span>
                  <div>
                    <strong>{x.name}</strong>
                    <small>
                      {x.scans} сканирований · {x.destination}
                    </small>
                  </div>
                  <b className={x.active ? "status" : "tag"}>
                    {x.active ? "Активно" : "Отключено"}
                  </b>
                </article>
              ))}
              {!data.devices.length && (
                <div className="branch-empty">
                  <Nfc />
                  <p>В филиале ещё нет NFC или QR.</p>
                  <Link href="/devices">Создать устройство</Link>
                </div>
              )}
            </section>
          </div>
        </>
      )}
    </SectionShell>
  );
}
