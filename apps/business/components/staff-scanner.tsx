"use client";
import { FormEvent, useEffect, useRef, useState } from "react";
import { Camera, CameraOff, CheckCircle2, Keyboard, LoaderCircle, RotateCcw, ScanLine, ShieldCheck, UserRound } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";

type Customer={id:string;firstName:string;lastName:string;phone:string;totalPoints:number;totalVisits:number;level:string};
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
  const [branches,setBranches]=useState<Branch[]>([]),[branchId,setBranchId]=useState(""),[customer,setCustomer]=useState<Customer|null>(null),[message,setMessage]=useState(""),[scanning,setScanning]=useState(false),[saving,setSaving]=useState(false),[success,setSuccess]=useState<VisitResult|null>(null);
  const scanner=useRef<{stop:()=>Promise<void>;clear:()=>void}|null>(null);
  useEffect(()=>{api<Branch[]>("/branches").then(items=>{const active=items.filter(x=>x.isActive);setBranches(active);setBranchId(active[0]?.id||"")}).catch(e=>setMessage(e.message));return()=>{void scanner.current?.stop().catch(()=>undefined)}},[]);
  async function stop(){if(scanner.current){await scanner.current.stop().catch(()=>undefined);scanner.current.clear();scanner.current=null}setScanning(false)}
  async function resolve(raw:string){
    const id=customerID(raw);
    if(!id){setMessage("Это не QR-код клиента Tappix");return}
    await stop();setMessage("");setSuccess(null);
    try{setCustomer(await api<Customer>(`/customers/${id}`))}catch(e){setMessage(e instanceof Error?e.message:"Клиент не найден")}
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
    try{const result=await api<VisitResult>("/visits",{method:"POST",body:JSON.stringify({customerId:customer.id,branchId,comment:"QR Scanner"})});setSuccess(result);setCustomer({...customer,totalVisits:result.totalVisits,totalPoints:result.balance})}catch(e){setMessage(e instanceof Error?e.message:"Не удалось отметить посещение")}finally{setSaving(false)}
  }
  function reset(){setCustomer(null);setSuccess(null);setMessage("")}
  function manual(e:FormEvent<HTMLFormElement>){e.preventDefault();const value=String(new FormData(e.currentTarget).get("code")||"");void resolve(value)}
  return <SectionShell active="/scanner" title="Сканер гостя" subtitle="Отметьте посещение за несколько секунд">
    <div className="scanner-layout">
      <section className="scanner-main">
        <div className="scanner-camera">
          <div id="staff-reader" className={scanning?"active":""}/>
          {!scanning&&<div className="scanner-placeholder"><span><ScanLine/></span><h2>Наведите камеру на QR гостя</h2><p>QR находится на цифровой бонусной карте клиента.</p><button onClick={()=>void start()}><Camera/>Включить камеру</button></div>}
          {scanning&&<button className="stop-camera" onClick={()=>void stop()}><CameraOff/>Остановить</button>}
        </div>
        <form className="scanner-manual" onSubmit={manual}><Keyboard/><label>Нет камеры?<input name="code" placeholder="Вставьте код из QR" required/></label><button>Найти</button></form>
      </section>
      <aside className="scan-result">
        {!customer&&!message&&<div className="scan-empty"><UserRound/><strong>Здесь появится гость</strong><p>Проверьте имя и подтвердите посещение.</p></div>}
        {message&&<div className="scan-error" role="alert"><CameraOff/><strong>Не удалось продолжить</strong><p>{message}</p><button onClick={reset}>Попробовать снова</button></div>}
        {customer&&<div className="guest-confirm">
          <div className="guest-confirm-head"><span>{customer.firstName.slice(0,1)}</span><div><small>КЛИЕНТ НАЙДЕН</small><h2>{customer.firstName} {customer.lastName}</h2><p>{customer.phone}</p></div><CheckCircle2/></div>
          <div className="guest-confirm-stats"><span><small>Посещений</small><strong>{customer.totalVisits}</strong></span><span><small>Баланс</small><strong>{customer.totalPoints}</strong></span><span><small>Статус</small><strong>{customer.level}</strong></span></div>
          {success?<div className="visit-success"><CheckCircle2/><h3>Посещение отмечено</h3><strong>+{success.pointsAdded} бонусов</strong><p>{success.reward?`Также выдана награда: ${success.reward}`:`Теперь посещений: ${success.totalVisits}`}</p><button onClick={reset}><RotateCcw/>Сканировать следующего</button></div>:<>
            <label className="scanner-branch">Точка посещения<select value={branchId} onChange={e=>setBranchId(e.target.value)}>{branches.map(x=><option value={x.id} key={x.id}>{x.name}</option>)}</select></label>
            <button className="confirm-visit" disabled={saving||!branchId} onClick={()=>void addVisit()}>{saving?<><LoaderCircle className="spin"/>Сохраняем…</>:<><CheckCircle2/>Отметить посещение</>}</button>
            <p className="scanner-safe"><ShieldCheck/>Защита от двойного начисления включена</p>
          </>}
        </div>}
      </aside>
    </div>
  </SectionShell>
}
