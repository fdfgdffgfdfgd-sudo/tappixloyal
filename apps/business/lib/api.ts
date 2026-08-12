import { csrfHeaders } from "./csrf";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

export async function api<T>(path:string, init:RequestInit={}) {
	const makeHeaders=()=>({...(init.body instanceof FormData?{}:{"Content-Type":"application/json"}),...csrfHeaders(),...init.headers});
	let response = await fetch(`${API_URL}${path}`, {...init,credentials:"include",headers:makeHeaders()});
	if(response.status===401){const refreshed=await fetch(`${API_URL}/auth/refresh`,{method:"POST",credentials:"include",headers:csrfHeaders()});if(refreshed.ok)response=await fetch(`${API_URL}${path}`,{...init,credentials:"include",headers:makeHeaders()})}
  const result = await response.json();
  if (response.status===401){window.location.href="/login"}
  if (!result.success) throw new Error(result.error?.message ?? "Ошибка запроса");
  return result.data as T;
}

export async function logout(){await fetch(`${API_URL}/auth/logout`,{method:"POST",credentials:"include",headers:csrfHeaders()}).catch(()=>{});window.location.href="/login"}

export async function download(path:string, filename:string){const response=await fetch(`${API_URL}${path}`,{credentials:"include"});if(!response.ok)throw new Error("Не удалось скачать файл");const blob=await response.blob();const url=URL.createObjectURL(blob);const link=document.createElement("a");link.href=url;link.download=filename;link.click();URL.revokeObjectURL(url)}
