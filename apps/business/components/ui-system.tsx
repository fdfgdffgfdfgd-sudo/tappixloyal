import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { AlertCircle, CheckCircle2, Info, LockKeyhole, PackageOpen, RefreshCw } from "lucide-react";

export function PageHeader({ eyebrow, title, subtitle, leading, actions }: { eyebrow?: string; title: string; subtitle?: string; leading?: ReactNode; actions?: ReactNode }) {
  return <header className="ui-page-header">
    {leading}
    <div className="ui-page-heading">{eyebrow&&<span>{eyebrow}</span>}<h1>{title}</h1>{subtitle&&<p>{subtitle}</p>}</div>
    {actions&&<div className="ui-page-actions">{actions}</div>}
  </header>;
}

export function SectionHeader({ title, description, action }: { title: string; description?: string; action?: ReactNode }) {
  return <header className="ui-section-header"><div><h2>{title}</h2>{description&&<p>{description}</p>}</div>{action}</header>;
}

export function StatusBadge({ tone="neutral", children }: { tone?: "neutral"|"success"|"warning"|"danger"|"locked"; children: ReactNode }) {
  return <span className={`ui-status ui-status-${tone}`}>{tone==="locked"&&<LockKeyhole/>}{children}</span>;
}

export function InfoCallout({ title, children, tone="info" }: { title: string; children: ReactNode; tone?: "info"|"success"|"warning" }) {
  const Icon=tone==="success"?CheckCircle2:tone==="warning"?AlertCircle:Info;
  return <aside className={`ui-callout ui-callout-${tone}`}><Icon/><div><strong>{title}</strong><p>{children}</p></div></aside>;
}

export function EmptyState({ icon:Icon=PackageOpen, title, description, action }: { icon?:LucideIcon; title:string; description:string; action?:ReactNode }) {
  return <div className="ui-empty"><span><Icon/></span><h3>{title}</h3><p>{description}</p>{action}</div>;
}

export function ErrorState({ title="Не удалось загрузить данные", description, retry }: { title?:string; description?:string; retry?:()=>void }) {
  return <div className="ui-error" role="alert"><span><AlertCircle/></span><h3>{title}</h3>{description&&<p>{description}</p>}{retry&&<button type="button" onClick={retry}><RefreshCw/>Повторить</button>}</div>;
}

export function Skeleton({ lines=3 }: { lines?:number }) {
  return <div className="ui-skeleton" aria-label="Загрузка" aria-busy="true">{Array.from({length:lines},(_,index)=><i key={index}/>)}</div>;
}
