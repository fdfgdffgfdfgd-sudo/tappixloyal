import { Check, Gift, QrCode } from "lucide-react";

export function CustomerCardPreview({business="Ваш бизнес",color="var(--accent)",progress=4,target=5,reward="Награда"}:{business?:string;color?:string;progress?:number;target?:number;reward?:string}){
 const remaining=Math.max(0,target-progress);
 return <div className="customer-card-preview" style={{"--preview-brand":color} as React.CSSProperties} aria-label="Предпросмотр карты клиента"><header><span>{business.slice(0,1)}</span><div><strong>{business}</strong><small>Ваша карта клиента</small></div></header><section><small>ВАШ ПРОГРЕСС</small><h3>{progress} из {target} посещений</h3><div>{Array.from({length:target},(_,index)=><i className={index<progress?"done":""} key={index}>{index<progress?<Check/>:null}</i>)}</div><strong>{remaining===0?"Награда доступна":remaining===1?"Осталось 1 посещение":`Осталось ${remaining} посещения`}</strong><p><Gift/>{reward}</p><button type="button"><QrCode/>Показать карту на кассе</button></section></div>
}
