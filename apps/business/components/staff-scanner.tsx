"use client";
import { FormEvent, useEffect, useRef, useState } from "react";
import { AlertTriangle, Camera, CameraOff, CheckCircle2, Gift, Keyboard, LoaderCircle, RotateCcw, ScanLine, ShieldCheck, UserRound, X } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";

type Customer={id:string;firstName:string;lastName:string;phone?:string;phoneMasked?:string;totalPoints:number;totalVisits:number;level?:string};
type RewardProgress={name:string;currentValue:number;targetValue:number;status:string};
type Branch={id:string;name:string;address:string;isActive:boolean};
type VisitResult={id:string;pointsAdded:number;balance:number;totalVisits:number;reward?:string};

function customerID(raw:string){
  const value=raw.trim();
  const match=value.match(/^tappix:customer:[a-z0-9-]+:([0-9a-f-]{36})$/i);
  if(match)return match[1];
  if(/^[0-9a-f-]{36}$/i.test(value))return value;
  return "";
}

export function StaffScanner(){
  const [branches,setBranches]=useState<Branch[]>([]),[branchId,setBranchId]=useState(""),[customer,setCustomer]=useState<Customer|null>(null),[progress,setProgress]=useState<RewardProgress|null>(null),[message,setMessage]=useState(""),[scanning,setScanning]=useState(false),[saving,setSaving]=useState(false),[resolving,setResolving]=useState(false),[branchesLoading,setBranchesLoading]=useState(true),[success,setSuccess]=useState<VisitResult|null>(null),[confirming,setConfirming]=useState(false);
  const scanner=useRef<{stop:()=>Promise<void>;clear:()=>void}|null>(null);
  const resolvingLock=useRef(false),manualInput=useRef<HTMLInputElement>(null),confirmDialog=useRef<HTMLElement>(null);
  useEffect(()=>{api<Branch[]>("/branches").then(items=>{const active=items.filter(x=>x.isActive);setBranches(active);setBranchId(active[0]?.id||"");if(!active.length)setMessage("Нет активного филиала. Попросите владельца включить филиал в настройках.")}).catch(e=>setMessage(e.message)).finally(()=>setBranchesLoading(false));return()=>{void scanner.current?.stop().catch(()=>undefined)}},[]);
  async function stop(){if(scanner.current){await scanner.current.stop().catch(()=>undefined);scanner.current.clear();scanner.current=null}setScanning(false)}
  async function resolve(raw:string){
    if(resolvingLock.current)return;resolvingLock.current=true;setResolving(true);
    let id=customerID(raw);const entered=raw.replace(/\s/g,"");
    if(!id&&/^\d{6}$/.test(entered)){try{const found=await api<Customer>("/staff/customers/lookup",{method:"POST",body:JSON.stringify({code:entered})});const rewards=await api<RewardProgress[]>(`/customers/${found.id}/reward-progress`);await stop();setMessage("");setSuccess(null);setCustomer(found);setProgress(rewards[0]||null);resolvingLock.current=false;setResolving(false);return}catch(e){setMessage(e instanceof Error?e.message:"Клиент не найден");resolvingLock.current=false;setResolving(false);return}}
    if(!id&&raw.trim().length>=2){try{const result=await api<{items:Customer[]}>(`/customers?search=${encodeURIComponent(raw.trim())}&limit=2`);if(result.items.length===1)id=result.items[0].id;else if(result.items.length>1){setMessage("Найдено несколько клиентов. Уточните телефон или отсканируйте QR.");resolvingLock.current=false;setResolving(false);return}}catch{}}
    if(!id){setMessage("Клиент не найден. Введите полный телефон, имя или отсканируйте QR карты.");resolvingLock.current=false;setResolving(false);return}
    await stop();setMessage("");setSuccess(null);
    try{const [found,rewards]=await Promise.all([api<Customer>(`/customers/${id}`),api<RewardProgress[]>(`/customers/${id}/reward-progress`)]);setCustomer(found);setProgress(rewards[0]||null)}catch(e){setMessage(e instanceof Error?e.message:"Клиент не найден")}finally{resolvingLock.current=false;setResolving(false)}
  }
  async function start(){
    setMessage("");setCustomer(null);setSuccess(null);
    try{
      const {Html5Qrcode}=await import("html5-qrcode");
      const instance=new Html5Qrcode("staff-reader");scanner.current=instance;setScanning(true);
      await instance.start({facingMode:"environment"},{fps:10,qrbox:{width:240,height:240},aspectRatio:1},decoded=>void resolve(decoded),()=>undefined);
    }catch{setScanning(false);scanner.current=null;setMessage("Не удалось открыть камеру. Разрешите доступ или используйте ручной ввод.")}
  }
  async function addVisit(){
    if(!customer||!branchId)return;setSaving(true);setMessage("");
    try{const result=await api<VisitResult>("/visits",{method:"POST",body:JSON.stringify({customerId:customer.id,branchId,comment:"Staff Mode"})});const rewards=await api<RewardProgress[]>(`/customers/${customer.id}/reward-progress`);setSuccess(result);setCustomer({...customer,totalVisits:result.totalVisits,totalPoints:result.balance});setProgress(rewards[0]||null)}catch(e){setMessage(e instanceof Error?e.message:"Не удалось отметить посещение")}finally{setSaving(false);setConfirming(false)}
  }
  function reset(){setCustomer(null);setProgress(null);setSuccess(null);setMessage(branches.length?"":"Нет активного филиала. Попросите владельца включить филиал в настройках.");requestAnimationFrame(()=>manualInput.current?.focus())}
  function manual(e:FormEvent<HTMLFormElement>){e.preventDefault();const value=String(new FormData(e.currentTarget).get("code")||"");void resolve(value)}
  return <SectionShell active="/scanner" title="Сканер гостя" subtitle="Отметьте посещение за несколько секунд">
    <section className="staff-mode-banner"><div><ShieldCheck/><span><small>STAFF MODE</small><strong>Только операции у кассы</strong></span></div><p>Нет доступа к аналитике, тарифам и массовым рассылкам.</p><b>{branchId?branches.find(item=>item.id===branchId)?.name:"Филиал не выбран"}</b></section>
    <ol className="scanner-steps" aria-label="Этапы отметки посещения"><li className={!customer?"current":"done"}><span>1</span>Сканирование</li><li className={customer&&!success?"current":success?"done":""}><span>2</span>Проверка гостя</li><li className={success?"current":""}><span>3</span>Готово</li></ol>
    <div className="scanner-layout">
      <section className="scanner-main">
        <div className="scanner-camera">
          <div id="staff-reader" className={scanning?"active":""}/>
          {!scanning&&<div className="scanner-placeholder"><span>{resolving?<LoaderCircle className="spin"/>:<ScanLine/>}</span><h2>{resolving?"Проверяем карту…":"Наведите камеру на QR гостя"}</h2><p>{resolving?"Находим клиента и его актуальный баланс.":"QR находится на цифровой бонусной карте клиента."}</p><button type="button" disabled={resolving||branchesLoading||!branches.length} onClick={()=>void start()}><Camera/>Включить камеру</button></div>}
          {scanning&&<button type="button" className="stop-camera" onClick={()=>void stop()}><CameraOff/>Остановить</button>}
        </div>
        <form className="scanner-manual" onSubmit={manual}><Keyboard/><label>Введите код клиента<input ref={manualInput} name="code" inputMode="numeric" autoComplete="off" pattern="[0-9 ]{6,7}" placeholder="482 731" maxLength={7} required/></label><button disabled={resolving}>{resolving?"Проверяем…":"Найти клиента"}</button></form>
      </section>
      <aside className="scan-result">
        {!customer&&!message&&<div className="scan-empty"><UserRound/><strong>Здесь появится гость</strong><p>Проверьте имя и подтвердите посещение.</p></div>}
        {message&&<div className="scan-error" role="alert"><AlertTriangle/><strong>Не удалось продолжить</strong><p>{message}</p><button type="button" onClick={reset}>Попробовать снова</button></div>}
        {customer&&<div className="guest-confirm">
          <div className="guest-confirm-head"><span>{customer.firstName.slice(0,1)}</span><div><small>КЛИЕНТ НАЙДЕН</small><h2>{customer.firstName} {customer.lastName}</h2><p>{customer.phoneMasked||customer.phone}</p></div><CheckCircle2/></div>
          {progress?<section className="staff-program-summary"><Gift/><div><small>ПРОГРАММА</small><strong>{progress.currentValue} из {progress.targetValue} посещений</strong><p>Следующая награда: {progress.name}</p></div></section>:<div className="guest-confirm-stats"><span><small>Посещений</small><strong>{customer.totalVisits}</strong></span><span><small>Баланс</small><strong>{customer.totalPoints}</strong></span></div>}
          {success?<div className="visit-success" role="status" aria-live="polite"><CheckCircle2/><h3>Посещение отмечено</h3><strong>+{success.pointsAdded} бонусов</strong><p>{success.reward?`Также выдана награда: ${success.reward}`:`Теперь посещений: ${success.totalVisits}`}</p><button type="button" onClick={reset}><RotateCcw/>Сканировать следующего</button></div>:<>
            <label className="scanner-branch">Точка посещения<select value={branchId} onChange={e=>setBranchId(e.target.value)}>{branches.map(x=><option value={x.id} key={x.id}>{x.name}</option>)}</select></label>
            <button type="button" className="confirm-visit" disabled={saving||branchesLoading||!branchId} onClick={()=>setConfirming(true)}><CheckCircle2/>Отметить посещение</button>
            <p className="scanner-safe"><ShieldCheck/>Защита от двойного начисления включена</p>
          </>}
        </div>}
      </aside>
    </div>
    {confirming&&<div className="staff-confirm-backdrop" role="presentation" onMouseDown={()=>setConfirming(false)}><section ref={confirmDialog} className="staff-confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="visit-confirm-title" onMouseDown={event=>event.stopPropagation()} onKeyDown={event=>{if(event.key==="Escape")setConfirming(false);if(event.key==="Tab"){const controls=Array.from(confirmDialog.current?.querySelectorAll<HTMLElement>('button:not([disabled])')||[]);if(!controls.length)return;const first=controls[0],last=controls[controls.length-1];if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus()}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus()}}}}><button className="staff-confirm-close" aria-label="Закрыть" onClick={()=>setConfirming(false)}><X/></button><span><CheckCircle2/></span><h2 id="visit-confirm-title">Отметить посещение?</h2><p><strong>{customer?.firstName} {customer?.lastName}</strong><br/>{branches.find(item=>item.id===branchId)?.name}</p><div><button type="button" onClick={()=>setConfirming(false)}>Отмена</button><button type="button" disabled={saving} autoFocus onClick={()=>void addVisit()}>{saving?<><LoaderCircle className="spin"/>Сохраняем…</>:"Да, отметить"}</button></div></section></div>}
  </SectionShell>
}
