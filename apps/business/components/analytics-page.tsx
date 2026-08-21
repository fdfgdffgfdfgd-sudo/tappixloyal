"use client";
import { useEffect, useState } from "react";
import { BarChart3, AlertTriangle, Banknote, Building2, CreditCard, Crown, Clock3, Gift, Repeat2, Send, Star, UserX, UserCheck, TrendingUp, Users } from "lucide-react";
import { api } from "@/lib/api";
import { SectionShell } from "./section-shell";
import Link from "next/link";
import { Notice } from "./management-shared";

type AnalyticsData = {
  days: number;
  totals: { customers: number; visits: number; pointsIssued: number; pointsRedeemed: number; outstanding: number };
  previous: { visits: number; active: number; new: number; pointsIssued: number };
  series: { date: string; customers: number; visits: number; points: number; firstVisits: number; repeatVisits: number }[];
  audience: {
    active: number;
    returning: number;
    repeatActive: number;
    frequent: number;
    loyal: number;
    atRisk: number;
    new: number;
    retentionRate: number;
    averageVisits: number;
  };
  topCustomers: {
    id: string;
    name: string;
    visits: number;
    points: number;
    level: string;
  }[];
  peakHour: number;
};
type AnalyticsSubscription = { plan: string; status: string };
type ProAnalytics = {
  currency: string;
  repeatPurchase: { windows: { days: number; customers: number; repeatCustomers: number; repeatPurchaseRate: number }[]; averageDaysToSecondPurchase: number; secondPurchaseConversion: number };
  averageCheck: { overall: number; participants: number; anonymous: number; newCustomers: number; repeatCustomers: number };
  ltv: { type: string; customers: number; totalRevenue: number; average: number; median: number; maximum: number };
  branches: { id: string; name: string; transactions: number; customers: number; revenue: number; averageCheck: number }[];
  rfm: { segments: { code: string; name: string; churnRisk: string; customers: number; revenue: number; averageLTV: number }[] };
};
type BonusLiability = { currency: string; issued: number; active: number; redeemed: number; expired: number; liability: number; expectedRedemptionCost: number };
type BusinessOutcomes = {days:number;retention:{returnedCustomers:number;repeatVisits:number};automations:{messages:number;reachedCustomers:number;returnedCustomers:number;attributedRevenue:number};referrals:{newCustomers:number;repeatCustomers:number;revenue:number};rewards:{bestName:string;redemptions:number};revenue:{members:number;campaignAttributed:number};previous:{returnedCustomers:number;automationReturned:number;automationRevenue:number;referralCustomers:number;referralRevenue:number;rewardRedemptions:number;memberRevenue:number};branches:{id:string;name:string;customers:number;returnedCustomers:number;visits:number;revenue:number;rewards:number}[]};
function MetricDelta({ current, previous }: { current: number; previous: number }) {
  const value = previous === 0 ? (current > 0 ? 100 : 0) : ((current - previous) / previous) * 100;
  return <small className={value >= 0 ? "metric-delta up" : "metric-delta down"}>{value >= 0 ? "+" : ""}{value.toFixed(0)}% к прошлому периоду</small>;
}
function OutcomeDelta({current,previous}:{current:number;previous:number}){
  if(previous===0)return current>0?<small className="outcome-delta new">Новый результат за период</small>:<small className="outcome-delta neutral">Без изменений</small>;
  const value=(current-previous)/previous*100;
  return <small className={`outcome-delta ${value>=0?"up":"down"}`}>{value>=0?"↑":"↓"} {Math.abs(value).toFixed(0)}% к прошлому периоду</small>;
}
export function AnalyticsPage() {
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [subscription, setSubscription] = useState<AnalyticsSubscription | null>(null);
  const [proData, setProData] = useState<ProAnalytics | null>(null);
  const [liability, setLiability] = useState<BonusLiability | null>(null);
  const [outcomes, setOutcomes] = useState<BusinessOutcomes | null>(null);
  const [period, setPeriod] = useState("month");
  const [branchId, setBranchId] = useState("");
  const [branches, setBranches] = useState<{id:string;name:string}[]>([]);
  const [msg, setMsg] = useState("");
  useEffect(() => {
    setData(null);setMsg("");
    const days=period==="week"?7:period==="quarter"?90:30;
    const branchQuery = branchId ? `&branchId=${encodeURIComponent(branchId)}` : "";
    Promise.all([api<AnalyticsData>(`/analytics?period=${period}${branchQuery}`),api<AnalyticsSubscription>("/subscription"),api<BusinessOutcomes>(`/analytics/outcomes?days=${days}${branchId ? `&branchId=${encodeURIComponent(branchId)}` : ""}`), branches.length ? Promise.resolve([] as {id:string;name:string}[]) : api<{id:string;name:string}[]>("/branches")])
      .then(([analytics, plan, result, availableBranches]) => {setData(analytics);setSubscription(plan);setOutcomes(result);if(availableBranches.length)setBranches(availableBranches);const normalized=plan.plan.toLowerCase()==="business"||plan.plan.toLowerCase()==="growth"?"growth":plan.plan.toLowerCase();if(normalized==="pro")return Promise.all([api<ProAnalytics>("/analytics/business"),api<BonusLiability>("/analytics/bonus-liability")]).then(([business,bonus])=>{setProData(business);setLiability(bonus)});setProData(null);setLiability(null)})
      .catch((e) => setMsg(e.message));
  }, [period, branchId]);
  const tier = subscription?.plan.toLowerCase()==="business"?"growth":subscription?.plan.toLowerCase()||"starter";
  const tierName = tier==="pro"?"Pro":tier==="growth"?"Growth":"Starter";
  const money = (value:number) => `${Math.round(value).toLocaleString("ru-RU")} ₸`;
  const max = Math.max(1, ...(data?.series.map((x) => x.visits) || [1]));
  return (
    <SectionShell
      active="/analytics"
      title="Аналитика"
      subtitle="Рост клиентов, посещения и начисления"
    >
      <Notice text={msg} />
      <section className={`analytics-plan analytics-plan-${tier}`}><div><span>{tier==="pro"?<Crown/>:tier==="growth"?<TrendingUp/>:<BarChart3/>}</span><div><small>АНАЛИТИКА {tierName.toLocaleUpperCase("ru-RU")}</small><h2>{tier==="pro"?"Финансовый центр сети":tier==="growth"?"Центр удержания клиентов":"Пульс программы лояльности"}</h2><p>{tier==="pro"?"Выручка, LTV, повторные покупки и обязательства по бонусам.":tier==="growth"?"Сегменты, риск ухода и конкретные аудитории для возврата.":"Только главные показатели без сложных отчётов."}</p></div></div><Link href="/subscription">{tier==="pro"?"Ваш максимальный тариф":"Сравнить тарифы"}</Link></section>
      <div className="toolbar">
        <span>Данные в реальном времени</span>
        <select aria-label="Период аналитики" value={period} onChange={(e) => setPeriod(e.target.value)}>
          <option value="week">7 дней</option>
          <option value="month">30 дней</option>
          <option value="quarter">90 дней</option>
        </select>
        {branches.length > 1 && <select aria-label="Филиал аналитики" value={branchId} onChange={(e) => setBranchId(e.target.value)}><option value="">Все филиалы</option>{branches.map(branch=><option key={branch.id} value={branch.id}>{branch.name}</option>)}</select>}
      </div>
      {data && (
        <>
          {outcomes&&<section className="business-outcomes"><header><div><small>РЕЗУЛЬТАТ ПРОГРАММЫ</small><h2>Что лояльность дала бизнесу?</h2><p>Только подтверждённые события за {outcomes.days} дней. Выручка без контрольной группы называется атрибутированной.</p></div><TrendingUp/></header><div>
            <article><span><Repeat2/></span><div><small>Возвращаются ли клиенты?</small><strong>{outcomes.retention.returnedCustomers} клиентов вернулись</strong><p>{outcomes.retention.repeatVisits} повторных посещений за период</p><OutcomeDelta current={outcomes.retention.returnedCustomers} previous={outcomes.previous.returnedCustomers}/></div></article>
            <article><span><Send/></span><div><small>Что дали автоматизации?</small><strong>{outcomes.automations.returnedCustomers} клиентов вернулись</strong><p>{outcomes.automations.attributedRevenue>0?`${money(outcomes.automations.attributedRevenue)} атрибутированной выручки`:`${outcomes.automations.reachedCustomers} клиентов получили сообщение`}</p><OutcomeDelta current={outcomes.automations.returnedCustomers} previous={outcomes.previous.automationReturned}/></div></article>
            <article><span><Users/></span><div><small>Работают ли рекомендации?</small><strong>{outcomes.referrals.newCustomers} новых клиентов</strong><p>{outcomes.referrals.repeatCustomers} уже совершили повторную покупку</p><OutcomeDelta current={outcomes.referrals.newCustomers} previous={outcomes.previous.referralCustomers}/></div></article>
            <article><span><Gift/></span><div><small>Какая награда работает лучше?</small><strong>{outcomes.rewards.bestName||"Пока недостаточно данных"}</strong><p>{outcomes.rewards.redemptions?`${outcomes.rewards.redemptions} использований`:`Появится после первого погашения`}</p><OutcomeDelta current={outcomes.rewards.redemptions} previous={outcomes.previous.rewardRedemptions}/></div></article>
          </div>{outcomes.branches.length>1&&<section className="outcome-branches"><header><div><small>ФИЛИАЛЫ</small><h3>Где программа лучше возвращает клиентов?</h3></div><Link href="/branches">Управлять филиалами</Link></header><div>{outcomes.branches.slice(0,4).map((branch,index)=><article key={branch.id}><b>{index+1}</b><span><strong>{branch.name}</strong><small>{branch.returnedCustomers} вернулись · {branch.visits} посещений</small></span><div><strong>{money(branch.revenue)}</strong><small>{branch.rewards} наград выдано</small></div></article>)}</div></section>}</section>}
          <div className="insight-metrics">
            <article>
              <Users />
              <span>Активных гостей</span>
              <strong>{data.audience.active}</strong>
              <MetricDelta current={data.audience.active} previous={data.previous.active} />
            </article>
            <article>
              <Building2 />
              <span>Посещений за период</span>
              <strong>{data.totals.visits}</strong>
              <MetricDelta current={data.totals.visits} previous={data.previous.visits} />
            </article>
            {tier==="starter"&&<article>
              <UserCheck />
              <span>Новых гостей</span>
              <strong>{data.audience.new}</strong>
              <MetricDelta current={data.audience.new} previous={data.previous.new} />
            </article>}
            {tier!=="starter"&&<article>
              <Repeat2 />
              <span>Возвращаются</span>
              <strong>{data.audience.retentionRate.toFixed(0)}%</strong>
              <small>
                {data.audience.returning} клиентов с повторным визитом
              </small>
            </article>}
            {tier!=="starter"&&<article className="risk-metric">
              <AlertTriangle />
              <span>Риск ухода</span>
              <strong>{data.audience.atRisk}</strong>
              <small>Не возвращались более 45 дней</small>
            </article>}
          </div>
          {tier==="starter"&&<section className="starter-pulse"><div><small>ПУЛЬС ЗА ПЕРИОД</small><strong>{Math.min(100,Math.round(data.audience.retentionRate*.6+Math.min(40,data.audience.active*2)))}</strong><span>из 100</span></div><div><h2>{data.totals.visits?"Программа работает":"Начните собирать визиты"}</h2><p>{data.audience.new>0?`${data.audience.new} новых гостей уже в базе. Следующая цель — вернуть их повторно.`:"Активируйте NFC/QR и зарегистрируйте первых гостей."}</p><Link href={data.totals.visits?"/customers":"/devices"}>{data.totals.visits?"Открыть клиентов":"Настроить NFC/QR"}</Link></div></section>}
          <section className="analytics-answer">
            <TrendingUp />
            <div>
              <span>ГЛАВНЫЙ ВЫВОД</span>
              <h2>
                {data.audience.retentionRate >= 50
                  ? "Клиенты хорошо возвращаются"
                  : "Есть потенциал увеличить повторные визиты"}
              </h2>
              <p>
                {data.audience.atRisk > 0
                  ? `${data.audience.atRisk} клиентам стоит отправить персональное предложение.`
                  : data.audience.new > data.audience.repeatActive
                    ? "Новых гостей больше, чем повторных. Помогите им сделать второй визит."
                    : "Сейчас нет клиентов с высоким риском ухода."}
              </p>
            </div>
            <b>
              {data.audience.averageVisits.toFixed(1)}
              <small>визита в среднем</small>
            </b>
          </section>
          {tier!=="starter"&&<section className="loyalty-economy">
            <header>
              <div><span>ЭКОНОМИКА ПРОГРАММЫ</span><h2>Движение бонусов</h2></div>
              <small>Без выдуманной выручки: показываем только реальные операции Tappix</small>
            </header>
            <div>
              <article><span>Начислено</span><strong>+{data.totals.pointsIssued}</strong><small>за выбранный период</small></article>
              <article><span>Использовано</span><strong>−{data.totals.pointsRedeemed}</strong><small>погашено гостями</small></article>
              <article><span>Баланс у гостей</span><strong>{data.totals.outstanding}</strong><small>доступно сейчас</small></article>
              <article><span>Всего в базе</span><strong>{data.totals.customers}</strong><small>зарегистрированных гостей</small></article>
            </div>
          </section>}
          {tier!=="starter"&&<div className="analytics-business-grid">
            <section className="analytics-segments">
              <header>
                <div>
                  <span>АУДИТОРИЯ</span>
                  <h2>Сегменты клиентов</h2>
                </div>
              </header>
              <div>
                <article>
                  <i className="new" />
                  <span>
                    <strong>{data.audience.new}</strong>
                    <small>Новые</small>
                  </span>
                </article>
                <article>
                  <i className="active" />
                  <span>
                    <strong>{data.audience.frequent}</strong>
                    <small>Частые · 5+ визитов</small>
                  </span>
                </article>
                <article>
                  <i className="loyal" />
                  <span>
                    <strong>{data.audience.loyal}</strong>
                    <small>Постоянные · 10+ визитов</small>
                  </span>
                </article>
                <article>
                  <i className="risk" />
                  <span>
                    <strong>{data.audience.atRisk}</strong>
                    <small>Нужно вернуть</small>
                  </span>
                </article>
              </div>
            </section>
            <section className="analytics-top">
              <header>
                <div>
                  <span>ТОП ГОСТЕЙ</span>
                  <h2>Самые лояльные клиенты</h2>
                </div>
                <Clock3 />
                <small>
                  Пиковое время: {String(data.peakHour).padStart(2, "0")}:00
                </small>
              </header>
              {data.topCustomers.map((customer, index) => (
                <Link href={`/customers/${customer.id}`} key={customer.id}>
                  <b>{index + 1}</b>
                  <span>
                    <strong>{customer.name}</strong>
                    <small>
                      {customer.level} · {customer.points} бонусов
                    </small>
                  </span>
                  <i>{customer.visits} виз.</i>
                </Link>
              ))}
            </section>
          </div>}
          {tier==="growth"&&<section className="growth-actions"><header><small>УНИКАЛЬНО ДЛЯ GROWTH</small><h2>Готовые аудитории для роста</h2><p>Не просто цифры — группы клиентов, с которыми можно работать прямо сейчас.</p></header><div><Link href="/customers"><span><UserX/></span><strong>{data.audience.atRisk} нужно вернуть</strong><small>Не были более 45 дней</small></Link><Link href="/campaigns"><span><UserCheck/></span><strong>{data.audience.new} новых гостей</strong><small>Помогите совершить второй визит</small></Link><Link href="/customers"><span><Star/></span><strong>{data.audience.loyal} постоянных</strong><small>Предложите VIP-награду</small></Link></div></section>}
          {tier==="pro"&&proData&&liability&&<section className="pro-intelligence"><header><div><small>УНИКАЛЬНО ДЛЯ PRO</small><h2>Экономика лояльности</h2><p>Метрики строятся по реальным закрытым чекам из POS.</p></div><Crown/></header><div className="pro-finance-grid"><article><Banknote/><span>Выручка участников</span><strong>{money(proData.ltv.totalRevenue)}</strong><small>{proData.ltv.customers} покупателей</small></article><article><TrendingUp/><span>Historical LTV</span><strong>{money(proData.ltv.average)}</strong><small>медиана {money(proData.ltv.median)}</small></article><article><CreditCard/><span>Средний чек</span><strong>{money(proData.averageCheck.overall)}</strong><small>участники {money(proData.averageCheck.participants)}</small></article><article><Repeat2/><span>Repeat purchase 30 дней</span><strong>{(proData.repeatPurchase.windows.find(x=>x.days===30)?.repeatPurchaseRate||0).toFixed(1)}%</strong><small>до второй покупки {proData.repeatPurchase.averageDaysToSecondPurchase.toFixed(1)} дн.</small></article><article className="liability"><Gift/><span>Bonus liability</span><strong>{money(liability.liability)}</strong><small>ожидаемое погашение {money(liability.expectedRedemptionCost)}</small></article></div>{proData.branches.length>0?<div className="pro-branches"><h3>Филиалы по выручке</h3>{proData.branches.slice(0,5).map((branch,index)=><article key={branch.id||branch.name}><b>{index+1}</b><span><strong>{branch.name}</strong><small>{branch.transactions} чеков · средний {money(branch.averageCheck)}</small></span><em>{money(branch.revenue)}</em></article>)}</div>:<div className="pro-empty"><Building2/><div><strong>Подключите POS, чтобы увидеть экономику</strong><small>После импорта чеков появятся LTV, средний чек, repeat purchase и рейтинг филиалов.</small></div><Link href="/integrations">Подключить интеграцию</Link></div>}</section>}
          <div className="analytics-chart">
            <div className="chart-heading">
              <div><BarChart3 /><span><b>Новые и повторные визиты</b><small>Показывает, возвращаются ли гости после первого знакомства</small></span></div>
              <div className="chart-legend"><i className="first" />Первый визит <i className="repeat" />Повторный</div>
            </div>
            <div className="visit-bars">
              {data.series.map((x, i) => (
                <div key={i} title={`${new Date(x.date).toLocaleDateString("ru-RU")}: ${x.firstVisits} новых, ${x.repeatVisits} повторных`}>
                  <i className="repeat" style={{ height: `${(x.repeatVisits / max) * 100}%` }} />
                  <i className="first" style={{ height: `${Math.max(x.firstVisits ? 3 : 0, (x.firstVisits / max) * 100)}%` }} />
                </div>
              ))}
            </div>
            <footer><span>{new Date(data.series[0]?.date).toLocaleDateString("ru-RU", { day: "numeric", month: "short" })}</span><span>Сегодня</span></footer>
          </div>
        </>
      )}
    </SectionShell>
  );
}
