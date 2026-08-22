import Link from "next/link";
import { ArrowRight, CheckCircle2 } from "lucide-react";

export function OwnerContext({ label, title, detail, href, action }: { label: string; title: string; detail: string; href?: string; action?: string }) {
  return <section className="owner-context-block"><div><small>{label}</small><strong>{title}</strong><span>{detail}</span></div>{href && <Link href={href}>{action || "Открыть"}<ArrowRight /></Link>}</section>;
}

export function CustomerMobileCard({ id, name, phone, level, visits, points }: { id: string; name: string; phone: string; level: string; visits: number; points: number }) {
  return <Link className="owner-customer-card" href={`/customers/${id}`}><span className="owner-customer-avatar">{name.slice(0, 1)}</span><span><strong>{name}</strong><small>{phone} · {level}</small><em><b>{visits} посещений</b><b>{points} бонусов</b></em></span><ArrowRight /></Link>;
}

export function StaffStatus({ active }: { active: boolean }) {
  return <span className={`owner-status ${active ? "active" : "inactive"}`}><CheckCircle2 />{active ? "Активен" : "Отключён"}</span>;
}
