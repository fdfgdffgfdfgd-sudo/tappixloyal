"use client";
import Image from "next/image";
import { FormEvent, useEffect, useState } from "react";
import { Eye, EyeOff, LockKeyhole } from "lucide-react";
import { useRouter } from "next/navigation";
import { QRCodeSVG } from "qrcode.react";
import { csrfHeaders } from "@/lib/csrf";
import { API_URL as base } from "@/lib/api";

export function BusinessLogin() {
  const router = useRouter();
  const [msg, setMsg] = useState("");
  // Success and failure share one message slot; this says which it is.
  const [msgIsSuccess, setMsgIsSuccess] = useState(false);
  const [show, setShow] = useState(false);
  const [loading, setLoading] = useState(false);
  const [mode, setMode] = useState<"login" | "forgot" | "reset">("login");
  const [resetToken, setResetToken] = useState("");
	const [mfaChallenge,setMfaChallenge]=useState("");
	const [pendingCredentials,setPendingCredentials]=useState({email:"",password:""});
  const [mfaSetup,setMfaSetup]=useState<{secret:string;otpauthUri:string}|null>(null);
  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get("reset") || "";
    if (token) { setResetToken(token); setMode("reset"); }
  }, [router]);
  async function request(path: string, body: Record<string, unknown>, audience?:"platform") {
	const response = await fetch(`${base}${path}`, { method: "POST", credentials:"include", headers: { "Content-Type": "application/json", ...(audience?csrfHeaders(audience):{}) }, body: JSON.stringify(body) });
    const result = await response.json();
    if (!result.success) throw new Error(result.error?.message || "Ошибка запроса");
    return result.data;
  }
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); setLoading(true); setMsg(""); setMsgIsSuccess(false);
    try {
		const fields=Object.fromEntries(new FormData(e.currentTarget));
		const credentials=mfaChallenge?{...pendingCredentials,mfaCode:String(fields.mfaCode||""),mfaChallenge}:fields;
		const result = await request("/auth/login", credentials);
		if(result.mfaRequired){setPendingCredentials({email:String(fields.email||pendingCredentials.email),password:String(fields.password||pendingCredentials.password)});setMfaChallenge(result.mfaChallenge);setMsg("Введите код из приложения-аутентификатора");return}
		if(result.mfaSetupRequired){const setup=await request("/auth/mfa/setup?aud=platform",{},"platform");setMfaSetup(setup);setMsg("");return}
		router.replace(result.user.role === "super_admin" ? "/admin" : "/");
    } catch (error) { setMsg(error instanceof Error ? error.message : "Не удалось войти"); } finally { setLoading(false); }
  }
  async function enableMfa(e:FormEvent<HTMLFormElement>){e.preventDefault();setLoading(true);setMsg("");setMsgIsSuccess(false);try{const code=String(new FormData(e.currentTarget).get("code")||"");await request("/auth/mfa/enable?aud=platform",{code},"platform");router.replace("/admin")}catch(error){setMsg(error instanceof Error?error.message:"Не удалось включить MFA")}finally{setLoading(false)}}
  async function forgot(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); setLoading(true); setMsg(""); setMsgIsSuccess(false);
    try { await request("/auth/forgot-password", { email: String(new FormData(e.currentTarget).get("email") || "") }); setMsgIsSuccess(true); setMsg("Если аккаунт существует, ссылка отправлена на email. Она действует 30 минут."); }
    catch (error) { setMsg(error instanceof Error ? error.message : "Не удалось отправить ссылку"); } finally { setLoading(false); }
  }
  async function reset(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); setLoading(true); setMsg(""); setMsgIsSuccess(false); const data = new FormData(e.currentTarget); const password = String(data.get("password") || "");
    if (password !== data.get("confirmPassword")) { setMsg("Пароли не совпадают"); setLoading(false); return; }
    try { await request("/auth/reset-password", { token: resetToken, newPassword: password }); window.history.replaceState({}, "", "/login"); setMode("login"); setMsgIsSuccess(true); setMsg("Пароль изменён. Теперь войдите с новым паролем."); }
    catch (error) { setMsg(error instanceof Error ? error.message : "Не удалось изменить пароль"); } finally { setLoading(false); }
  }
  return <div className="business-login"><section><div className="login-art"><div className="portal-logo"><Image className="product-mark" src="/tappix-mark.png" alt="" aria-hidden width={32} height={32} priority/>Tappix</div><div><small className="login-kicker">Платформа лояльности</small><h1>Клиенты возвращаются.<br />Бизнес растёт.</h1><p>Программа лояльности, коммуникации и понятная аналитика в одном спокойном рабочем пространстве.</p></div><small>© 2026 Tappix Platform</small></div>
    {mode === "login" && !mfaSetup && <form onSubmit={submit}><span className="login-lock"><LockKeyhole /></span><h2>{mfaChallenge?"Подтверждение входа":"Вход в Tappix"}</h2><p>{mfaChallenge?"Введите одноразовый код MFA.":"Введите данные владельца или сотрудника."}</p>{msg && <div className={msgIsSuccess ? "login-notice" : "login-error"} role={msgIsSuccess ? "status" : "alert"}>{msg}</div>}{!mfaChallenge&&<><label>Email<input name="email" type="email" autoComplete="email" required /></label><label>Пароль<div className="password-field"><input name="password" type={show ? "text" : "password"} autoComplete="current-password" required /><button type="button" aria-label={show ? "Скрыть пароль" : "Показать пароль"} onClick={() => setShow(!show)}>{show ? <EyeOff /> : <Eye />}</button></div></label></>}{mfaChallenge&&<label>Код MFA<input name="mfaCode" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required autoFocus/></label>}<button className="login-submit" disabled={loading}>{loading ? "Проверяем…" : "Войти"}</button>{!mfaChallenge&&<button className="login-text-action" type="button" onClick={() => { setMode("forgot"); setMsg(""); }}>Забыли пароль?</button>}</form>}
    {mode === "login" && mfaSetup && <form onSubmit={enableMfa}><span className="login-lock"><LockKeyhole /></span><h2>Защитите Platform Console</h2><p>MFA обязательна для владельца платформы. Отсканируйте QR в приложении-аутентификаторе.</p>{msg&&<div className={msgIsSuccess ? "login-notice" : "login-error"} role={msgIsSuccess ? "status" : "alert"}>{msg}</div>}<div className="mfa-setup-qr"><QRCodeSVG value={mfaSetup.otpauthUri} size={168}/></div><details><summary>Ввести ключ вручную</summary><code>{mfaSetup.secret}</code></details><label>Код из приложения<input name="code" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required autoFocus/></label><button className="login-submit" disabled={loading}>{loading?"Проверяем…":"Включить MFA и продолжить"}</button></form>}
    {mode === "forgot" && <form onSubmit={forgot}><span className="login-lock"><LockKeyhole /></span><h2>Восстановление пароля</h2><p>Отправим одноразовую ссылку на email владельца или сотрудника.</p>{msg && <div className={msgIsSuccess ? "login-notice" : "login-error"} role={msgIsSuccess ? "status" : "alert"}>{msg}</div>}<label>Email<input name="email" type="email" autoComplete="email" required autoFocus /></label><button className="login-submit" disabled={loading}>{loading ? "Отправляем…" : "Получить ссылку"}</button><button className="login-text-action" type="button" onClick={() => { setMode("login"); setMsg(""); }}>Вернуться ко входу</button></form>}
    {mode === "reset" && <form onSubmit={reset}><span className="login-lock"><LockKeyhole /></span><h2>Новый пароль</h2><p>Минимум 8 символов. После смены все активные сессии завершатся.</p>{msg && <div className={msgIsSuccess ? "login-notice" : "login-error"} role={msgIsSuccess ? "status" : "alert"}>{msg}</div>}<label>Новый пароль<input name="password" type="password" autoComplete="new-password" minLength={8} required autoFocus /></label><label>Повторите пароль<input name="confirmPassword" type="password" autoComplete="new-password" minLength={8} required /></label><button className="login-submit" disabled={loading}>{loading ? "Сохраняем…" : "Изменить пароль"}</button></form>}
  </section></div>;
}
