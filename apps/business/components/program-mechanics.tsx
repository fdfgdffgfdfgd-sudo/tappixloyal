"use client";
import { useEffect, useState } from "react";
import { Check, Coins, Eye, Gift, Percent, QrCode, Save, Stamp } from "lucide-react";
import { api } from "@/lib/api";

type Mode="points"|"stamps"|"discount";
type Settings={loyaltyMode:Mode;stampsTarget:number;stampReward:string;discountStart:number;discountStep:number;discountMax:number;visitsPerStep:number;primaryColor?:string;[key:string]:unknown};
const defaults:Settings={loyaltyMode:"points",stampsTarget:6,stampReward:"Подарок",discountStart:3,discountStep:2,discountMax:15,visitsPerStep:3};
const modes=[
  {id:"points" as const,Icon:Coins,title:"Бонусная карта",help:"Баланс растёт после каждого визита",best:"Услуги и средний чек"},
  {id:"stamps" as const,Icon:Stamp,title:"Штамп-карта",help:"Каждый визит приближает подарок",best:"Кофейни и частые визиты"},
  {id:"discount" as const,Icon:Percent,title:"Растущая скидка",help:"Процент повышается с визитами",best:"Retail и салоны"},
];
export function ProgramMechanics(){
  const [value,setValue]=useState<Settings>(defaults),[saving,setSaving]=useState(false),[message,setMessage]=useState("");
  useEffect(()=>{api<Partial<Settings>>("/settings/guest-portal").then(x=>setValue({...defaults,...x})).catch(e=>setMessage(e.message))},[]);
  async function save(){setSaving(true);setMessage("");try{await api("/settings/guest-portal",{method:"PATCH",body:JSON.stringify(value)});setMessage("Механика опубликована в карте гостя")}catch(e){setMessage(e instanceof Error?e.message:"Не удалось сохранить")}finally{setSaving(false)}}
  const accent=value.primaryColor||"#6352ee";
  return <section className="mechanics-studio">
    <header><div><small>ШАГ 1 · МЕХАНИКА</small><h2>За что клиент захочет вернуться?</h2><p>Выберите один простой сценарий. Справа сразу видно, что получит клиент.</p></div>{message&&<span><Check/>{message}</span>}</header>
    <div className="mechanics-body">
      <div className="mechanics-config">
        <fieldset className="mechanics-choice"><legend>Выберите механику</legend><div className="mechanics-modes">{modes.map(({id,Icon,title,help,best})=><button type="button" aria-pressed={value.loyaltyMode===id} className={value.loyaltyMode===id?"active":""} onClick={()=>setValue({...value,loyaltyMode:id})} key={id}><span><Icon/></span><div><strong>{title}</strong><p>{help}</p><small>Подходит: {best.toLowerCase()}</small></div>{value.loyaltyMode===id&&<Check/>}</button>)}</div></fieldset>
        <div className="mechanics-fields">
          {value.loyaltyMode==="points"&&<div className="mechanics-explain"><Coins/><div><strong>Клиент копит бонусы после покупок</strong><p>Размер начислений задаётся во вкладке «Автоматизации». Баланс и его денежная ценность видны в карте.</p></div></div>}
          {value.loyaltyMode==="stamps"&&<><label>Сколько визитов до подарка<input type="number" min="2" max="30" value={value.stampsTarget} onChange={e=>setValue({...value,stampsTarget:Number(e.target.value)})}/><small>Обычно лучше работает короткая цель: 5–8 визитов</small></label><label>Подарок после заполнения карты<input value={value.stampReward} onChange={e=>setValue({...value,stampReward:e.target.value})} placeholder="Например, бесплатный кофе"/></label></>}
          {value.loyaltyMode==="discount"&&<div className="discount-field-grid"><label>Старт, %<input type="number" min="0" max="50" value={value.discountStart} onChange={e=>setValue({...value,discountStart:Number(e.target.value)})}/></label><label>Шаг, %<input type="number" min="1" max="20" value={value.discountStep} onChange={e=>setValue({...value,discountStep:Number(e.target.value)})}/></label><label>Каждые N визитов<input type="number" min="1" max="50" value={value.visitsPerStep} onChange={e=>setValue({...value,visitsPerStep:Number(e.target.value)})}/></label><label>Максимум, %<input type="number" min="1" max="70" value={value.discountMax} onChange={e=>setValue({...value,discountMax:Number(e.target.value)})}/></label></div>}
        </div>
        <div className="mechanics-action"><span><Eye/><small>Предпросмотр обновляется автоматически</small></span><button className="mechanics-save" disabled={saving} onClick={()=>void save()}><Save/>{saving?"Публикуем…":"Опубликовать программу"}</button></div>
      </div>
      <aside className="mechanics-preview"><header><div><small>КАРТА КЛИЕНТА</small><strong>Так её увидит гость</strong></div><span><i/>Живой предпросмотр</span></header><div className="phone-preview"><div className="phone-top"><i/><i/><i/></div><div className="mini-pass" style={{"--pass-accent":accent} as React.CSSProperties}><header><span>D</span><strong>Dentline</strong><i>{value.loyaltyMode==="stamps"?"ШТАМПЫ":value.loyaltyMode==="discount"?"СКИДКА":"БОНУСЫ"}</i></header>{value.loyaltyMode==="points"&&<div className="mini-balance"><small>ВАШ БАЛАНС</small><strong>420</strong><span>≈ 420 ₸ на следующую покупку</span></div>}{value.loyaltyMode==="stamps"&&<><div className="mini-stamps">{Array.from({length:Math.min(12,value.stampsTarget)},(_,i)=><span className={i<3?"filled":""} key={i}>{i<3?<Check/>:i+1}</span>)}</div><p className="mini-reward"><Gift/>Ещё {Math.max(value.stampsTarget-3,0)} визита до «{value.stampReward}»</p></>}{value.loyaltyMode==="discount"&&<div className="mini-discount"><small>ВАША СКИДКА</small><strong>{value.discountStart}%</strong><p>Следующий уровень после {value.visitsPerStep} визитов</p></div>}<footer><div><small>УЧАСТНИК</small><strong>Мадина Тест</strong></div><b aria-label="QR-код клиента"><QrCode/></b></footer></div></div><p><Check/>После публикации карта обновится у всех клиентов. Ссылка и QR останутся прежними.</p></aside>
    </div>
  </section>
}
