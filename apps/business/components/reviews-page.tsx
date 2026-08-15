"use client";
import { FormEvent, useEffect, useState } from "react";
import { Star } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { Notice } from "./management-shared";

type ReviewSettings = {
  gisUrl: string;
  googleUrl: string;
  yandexUrl: string;
  redirectThreshold: number;
  enabled: boolean;
};
export function ReviewsPage() {
  const [value, setValue] = useState<ReviewSettings>({
    gisUrl: "",
    googleUrl: "",
    yandexUrl: "",
    redirectThreshold: 4,
    enabled: false,
  });
  const [msg, setMsg] = useState("");
  useEffect(() => {
    api<ReviewSettings>("/reviews/settings")
      .then(setValue)
      .catch((e) => setMsg(e.message));
  }, []);
  async function save(e: FormEvent) {
    e.preventDefault();
    try {
      await api("/reviews/settings", {
        method: "PATCH",
        body: JSON.stringify(value),
      });
      setMsg("Настройки отзывов сохранены");
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка");
    }
  }
  return (
    <SectionShell
      active="/reviews"
      title="Отзывы"
      subtitle="2GIS, Google Maps и Яндекс Карты"
    >
      <Notice text={msg} />
      <form className="settings-card" onSubmit={save}>
        <div className="settings-title">
          <span>
            <Star />
          </span>
          <div>
            <h2>Перенаправление отзывов</h2>
            <p>Предлагайте довольным клиентам оставить публичный отзыв.</p>
          </div>
        </div>
        <div className="form-grid">
          <label>
            Ссылка 2GIS
            <input
              value={value.gisUrl}
              onChange={(e) => setValue({ ...value, gisUrl: e.target.value })}
            />
          </label>
          <label>
            Ссылка Google Maps
            <input
              value={value.googleUrl}
              onChange={(e) =>
                setValue({ ...value, googleUrl: e.target.value })
              }
            />
          </label>
          <label>
            Ссылка Яндекс
            <input
              value={value.yandexUrl}
              onChange={(e) =>
                setValue({ ...value, yandexUrl: e.target.value })
              }
            />
          </label>
          <label>
            Порог оценки
            <input
              type="number"
              min="1"
              max="5"
              step="0.5"
              value={value.redirectThreshold}
              onChange={(e) =>
                setValue({
                  ...value,
                  redirectThreshold: Number(e.target.value),
                })
              }
            />
          </label>
        </div>
        <label className="toggle-row">
          <input
            type="checkbox"
            checked={value.enabled}
            onChange={(e) => setValue({ ...value, enabled: e.target.checked })}
          />
          Модуль отзывов включён
        </label>
        <button className="primary-action">Сохранить</button>
      </form>
    </SectionShell>
  );
}
