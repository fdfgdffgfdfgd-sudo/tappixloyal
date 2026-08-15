"use client";
import { FormEvent, useEffect, useRef, useState } from "react";
import { Check, ShieldCheck, ShieldAlert } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import { Notice } from "./management-shared";

type Audit = {
  id: number;
  action: string;
  entityType: string;
  requestId: string;
  ip?: string;
  createdAt: string;
  user: string;
  company: string;
};
type OperationApproval={id:string;operation:"bonus.credit"|"bonus.debit";amount:number;reason:string;status:string;requestedAt:string;expiresAt:string;customerId:string;customer:string;branch:string;requester:string};
export function AuditPage() {
  const [items, setItems] = useState<Audit[]>([]);
  const [approvals,setApprovals]=useState<OperationApproval[]>([]);
  const [decision,setDecision]=useState<{id:string;value:"approved"|"rejected"}|null>(null);
  const [saving,setSaving]=useState(false);
  const [msg, setMsg] = useState("");
  const decisionDialogRef=useRef<HTMLFormElement>(null);
  const decisionTriggerRef=useRef<HTMLElement|null>(null);
  function load(){return Promise.all([api<Audit[]>("/audit"),api<OperationApproval[]>("/operation-approvals?status=pending")]).then(([audit,pending])=>{setItems(audit);setApprovals(pending)}).catch((e)=>setMsg(e.message))}
  useEffect(() => {void load()}, []);
  useEffect(()=>{
    if(!decision)return;
    decisionTriggerRef.current=document.activeElement as HTMLElement;
    const onKeyDown=(event:KeyboardEvent)=>{
      if(event.key==="Escape"){event.preventDefault();setDecision(null);return}
      if(event.key!=="Tab"||!decisionDialogRef.current)return;
      const focusable=Array.from(decisionDialogRef.current.querySelectorAll<HTMLElement>('button:not([disabled]), textarea:not([disabled])'));
      if(!focusable.length)return;
      const first=focusable[0];const last=focusable[focusable.length-1];
      if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus()}
      else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus()}
    };
    document.addEventListener("keydown",onKeyDown);
    return()=>{document.removeEventListener("keydown",onKeyDown);decisionTriggerRef.current?.focus()};
  },[decision]);
  async function submitDecision(event:FormEvent<HTMLFormElement>){event.preventDefault();if(!decision)return;const data=new FormData(event.currentTarget);setSaving(true);setMsg("");try{await api(`/operation-approvals/${decision.id}/decision`,{method:"POST",body:JSON.stringify({decision:decision.value,reason:String(data.get("reason")||"")})});setDecision(null);await load()}catch(error){setMsg(error instanceof Error?error.message:"Не удалось сохранить решение")}finally{setSaving(false)}}
  return (
    <SectionShell
      active="/audit"
      title="Журнал аудита"
      subtitle="Кто, когда и что изменил"
    >
      <Notice text={msg} />
      <section className="approval-queue"><header><div><small>КОНТРОЛЬ ОПЕРАЦИЙ</small><h2>Требуют вашего решения</h2><p>Крупные ручные начисления и списания не выполняются без подтверждения владельца.</p></div><b>{approvals.length}</b></header>{approvals.length?<div>{approvals.map(item=><article key={item.id}><span className={item.operation==="bonus.debit"?"debit":"credit"}>{item.operation==="bonus.debit"?"−":"+"}{item.amount.toLocaleString("ru-RU")}</span><div><strong>{item.customer}</strong><small>{item.branch} · сотрудник {item.requester}</small><p>{item.reason}</p><time>До {new Date(item.expiresAt).toLocaleString("ru-RU")}</time></div><nav><button onClick={()=>setDecision({id:item.id,value:"rejected"})}>Отклонить</button><button onClick={()=>setDecision({id:item.id,value:"approved"})}><Check/>Одобрить</button></nav></article>)}</div>:<div className="approval-empty"><ShieldCheck/><span><strong>Нет заявок на подтверждение</strong><small>Все крупные операции рассмотрены.</small></span></div>}</section>
      <div className="data-card">
        <table>
          <thead>
            <tr>
              <th>Время</th>
              <th>Пользователь</th>
              <th>Действие</th>
              <th>IP</th>
              <th>Request ID</th>
            </tr>
          </thead>
          <tbody>
            {items.map((x) => (
              <tr key={x.id}>
                <td>{new Date(x.createdAt).toLocaleString("ru-RU")}</td>
                <td>
                  <strong>{x.user}</strong>
                </td>
                <td>{x.action}</td>
                <td>{x.ip || "—"}</td>
                <td>
                  <code>{x.requestId}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!items.length && (
          <div className="zero">
            <strong>Журнал пока пуст</strong>
            <p>Новые изменения появятся здесь.</p>
          </div>
        )}
      </div>
      {decision&&<div className="approval-dialog-backdrop" role="presentation" onMouseDown={()=>setDecision(null)}><form ref={decisionDialogRef} className="approval-dialog" role="dialog" aria-modal="true" aria-labelledby="approval-dialog-title" aria-describedby="approval-dialog-description" onSubmit={submitDecision} onMouseDown={event=>event.stopPropagation()}><span>{decision.value==="approved"?<ShieldCheck/>:<ShieldAlert/>}</span><h2 id="approval-dialog-title">{decision.value==="approved"?"Одобрить операцию?":"Отклонить операцию?"}</h2><p id="approval-dialog-description">{decision.value==="approved"?"Бонусная операция будет выполнена сразу после подтверждения.":"Баланс клиента не изменится, сотрудник увидит отклонённую заявку."}</p><label>Причина решения<textarea name="reason" minLength={4} placeholder={decision.value==="approved"?"Проверил чек и подтверждаю":"Например: сумма не совпадает с чеком"} autoFocus required/></label><div><button type="button" onClick={()=>setDecision(null)}>Отмена</button><button disabled={saving}>{saving?"Сохраняем…":decision.value==="approved"?"Одобрить и выполнить":"Отклонить"}</button></div></form></div>}
    </SectionShell>
  );
}
