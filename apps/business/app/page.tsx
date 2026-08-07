"use client";
import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { ArrowRight, Check, CircleDot, Gift, Nfc, Plus, Sparkles, UserPlus, Users, X } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "@/components/section-shell";

type DashboardData = { customers:number; visitsToday:number; bonusIssued:number; bonusRedeemed:number; registrations:number; nfcConversion:number; latestCustomers:{id:string;name:string;phone:string;points:number;createdAt:string}[]; latestVisits:{id:string;customer:string;branch:string;points:number;createdAt:string}[]; onboarding:Record<string,boolean> };
type Customer = { id:string; firstName:string; lastName:string; phone:string };
type Branch = { id:string; name:string };
type Action = "customer"|"visit"|"bonus"|"nfc"|null;
const empty:DashboardData={customers:0,visitsToday:0,bonusIssued:0,bonusRedeemed:0,registrations:0,nfcConversion:0,latestCustomers:[],latestVisits:[],onboarding:{}};

export default function Dashboard(){
 const[data,setData]=useState(empty),[customers,setCustomers]=useState<Customer[]>([]),[branches,setBranches]=useState<Branch[]>([]),[action,setAction]=useState<Action>(null),[message,setMessage]=useState(""),[saving,setSaving]=useState(false);
 async function load(){try{const[d,c,b]=await Promise.all([api<DashboardData>("/dashboard"),api<{items:Customer[]}>("/customers?limit=100"),api<Branch[]>("/branches")]);setData(d);setCustomers(c.items);setBranches(b)}catch(e){setMessage(e instanceof Error?e.message:"Не удалось загрузить обзор")}}
 useEffect(()=>{void load()},[]);
 async function submit(e:FormEvent<HTMLFormElement>){e.preventDefault();setSaving(true);setMessage("");const form=Object.fromEntries(new FormData(e.currentTarget));try{
   if(action==="customer")await api("/customers",{method:"POST",body:JSON.stringify(form)});
   if(action==="visit")await api("/visits",{method:"POST",body:JSON.stringify(form)});
   if(action==="bonus")await api(`/customers/${form.customerId}/bonus`,{method:"POST",body:JSON.stringify({operation:"credit",amount:Number(form.amount),description:form.description})});
   if(action==="nfc")await api("/devices",{method:"POST",body:JSON.stringify({...form,kind:"nfc",destination:"join"})});
   setAction(null);setMessage("Действие выполнено");await load();
 }catch(error){setMessage(error instanceof Error?error.message:"Не удалось выполнить действие")}finally{setSaving(false)}}
 const steps=[['company','Компания настроена','/settings'],['branch','Добавьте филиал','/branches'],['team','Пригласите сотрудника','/employees'],['device','Активируйте NFC','/devices'],['firstCustomer','Получите первого клиента','/customers']] as const;
 const completed=steps.filter(([key])=>data.onboarding[key]).length;
 const metrics=[['Регистрации сегодня',data.registrations,'Новые клиенты'],['Посещения сегодня',data.visitsToday,'Отмеченные визиты'],['Начислено',data.bonusIssued,'Бонусов сегодня'],['Потрачено',data.bonusRedeemed,'Бонусов сегодня'],['NFC-конверсия',`${Math.min(data.nfcConversion,100).toFixed(1)}%`,'Касание → клиент']];
 return <SectionShell active="/" title="Обзор" subtitle="Главное о вашей программе лояльности сегодня">
   {message&&<div className="v2-notice" role="status">{message}</div>}
   <div className="v2-hero"><div><span>Сегодня</span><h2>Всё готово для следующего гостя</h2><p>Сканируйте карту, отмечайте визит — Tappix сам обновит бонусы, штампы или скидку.</p></div><Link className="dashboard-scan" href="/scanner"><Nfc/>Открыть сканер</Link></div>
   {completed<steps.length&&<section className="onboarding-card"><div className="onboarding-head"><div><span><Sparkles/></span><div><h2>Запустите Tappix</h2><p>{completed} из {steps.length} шагов выполнено</p></div></div><strong>{Math.round(completed/steps.length*100)}%</strong></div><div className="progress"><i style={{width:`${completed/steps.length*100}%`}}/></div><div className="onboarding-steps">{steps.map(([key,label,href])=><Link className={data.onboarding[key]?"done":""} href={href} key={key}>{data.onboarding[key]?<Check/>:<CircleDot/>}<span>{label}</span><ArrowRight/></Link>)}</div></section>}
   <div className="v2-metrics">{metrics.map(([label,value,help])=><article key={label}><p>{label}</p><strong>{value}</strong><small>{help}</small></article>)}</div>
   <section className="quick-card"><div><h2>Быстрые действия</h2><p>Частые операции в один клик</p></div><div className="quick-actions"><button onClick={()=>setAction("customer")}><UserPlus/><span><strong>Создать клиента</strong><small>Добавить в CRM</small></span></button><button onClick={()=>setAction("visit")}><Users/><span><strong>Добавить посещение</strong><small>Начислить автоматически</small></span></button><button onClick={()=>setAction("bonus")}><Gift/><span><strong>Начислить бонусы</strong><small>Ручная операция</small></span></button><button onClick={()=>setAction("nfc")}><Nfc/><span><strong>Создать NFC</strong><small>Новая точка касания</small></span></button></div></section>
   <div className="v2-feed-grid"><section className="feed-card"><div className="feed-head"><h2>Последние регистрации</h2><Link href="/customers">Все клиенты</Link></div>{data.latestCustomers.length?data.latestCustomers.map(x=><Link href={`/customers/${x.id}`} className="feed-row" key={x.id}><span>{x.name.slice(0,1)}</span><div><strong>{x.name}</strong><small>{x.phone}</small></div><time>{new Date(x.createdAt).toLocaleDateString("ru-RU")}</time></Link>):<div className="feed-empty">Пока нет регистраций</div>}</section><section className="feed-card"><div className="feed-head"><h2>Последние посещения</h2><Link href="/analytics">Аналитика</Link></div>{data.latestVisits.length?data.latestVisits.map(x=><div className="feed-row" key={x.id}><span><Check/></span><div><strong>{x.customer}</strong><small>{x.branch} · +{x.points} бонусов</small></div><time>{new Date(x.createdAt).toLocaleDateString("ru-RU")}</time></div>):<div className="feed-empty">Пока нет посещений</div>}</section></div>
   {action&&<div className="v2-dialog-backdrop" onMouseDown={()=>setAction(null)}><div className="v2-dialog" role="dialog" aria-modal="true" aria-labelledby="quick-title" onMouseDown={e=>e.stopPropagation()}><header><div><h2 id="quick-title">{{customer:"Новый клиент",visit:"Добавить посещение",bonus:"Начислить бонусы",nfc:"Создать NFC"}[action]}</h2><p>Заполните данные и подтвердите действие.</p></div><button aria-label="Закрыть" onClick={()=>setAction(null)}><X/></button></header><form onSubmit={submit}>
    {action==="customer"&&<><label>Имя<input name="firstName" required autoFocus/></label><label>Фамилия<input name="lastName"/></label><label>Телефон<input name="phone" type="tel" required placeholder="+7 700 000 00 00"/></label><label>Email<input name="email" type="email"/></label></>}
    {action==="visit"&&<><label>Клиент<select name="customerId" required autoFocus><option value="">Выберите клиента</option>{customers.map(x=><option value={x.id} key={x.id}>{x.firstName} {x.lastName} · {x.phone}</option>)}</select></label><label>Филиал<select name="branchId" required><option value="">Выберите филиал</option>{branches.map(x=><option value={x.id} key={x.id}>{x.name}</option>)}</select></label><label>Комментарий<textarea name="comment" rows={3}/></label></>}
    {action==="bonus"&&<><label>Клиент<select name="customerId" required autoFocus><option value="">Выберите клиента</option>{customers.map(x=><option value={x.id} key={x.id}>{x.firstName} {x.lastName}</option>)}</select></label><label>Количество<input name="amount" type="number" min="1" required/></label><label>Причина<input name="description" required placeholder="Компенсация или акция"/></label></>}
    {action==="nfc"&&<><label>Название<input name="name" required autoFocus placeholder="Стойка ресепшн"/></label><label>Филиал<select name="branchId" required><option value="">Выберите филиал</option>{branches.map(x=><option value={x.id} key={x.id}>{x.name}</option>)}</select></label></>}
    <div className="v2-dialog-actions"><button type="button" onClick={()=>setAction(null)}>Отмена</button><button className="primary" disabled={saving}>{saving?"Сохраняем…":"Подтвердить"}</button></div></form></div></div>}
 </SectionShell>
}
