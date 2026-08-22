"use client";
import { Check } from "lucide-react";

export type Customer = {
  id: string;
  firstName: string;
  lastName: string;
  phone: string;
  email?: string;
  birthday?: string;
  totalVisits: number;
  totalPoints: number;
  level: string;
  createdAt: string;
  favoriteBranch?: string;
  lastBranch?: string;
  lastVisit?: string;
  segment?: "new" | "active" | "loyal" | "at_risk" | "inactive";
  status?: "active" | "inactive";
};
export type Branch = {
  id: string;
  name: string;
  address: string;
  active: boolean;
  phone?: string;
};
export type Module = { code: string; name: string; core: boolean; enabled: boolean };
export type Loyalty = {
  welcomeBonus: number;
  pointsPerVisit: number;
  birthdayBonus: number;
  visitsForReward: number;
  rewardName: string;
};
export function Notice({ text }: { text: string }) {
  return text ? (
    <div className="notice" role="status">
      <Check size={17} />
      {text}
    </div>
  ) : null;
}
