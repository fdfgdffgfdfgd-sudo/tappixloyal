export function csrfToken(audience:"business"|"guest"|"platform"="business") {
	if (typeof document === "undefined") return "";
	const name=audience==="guest"?"tappix_guest_csrf":audience==="platform"?"tappix_platform_csrf":"tappix_csrf";
	const value=document.cookie.split("; ").find(item=>item.startsWith(`${name}=`));
	return value?decodeURIComponent(value.slice(name.length+1)):"";
}

export function csrfHeaders(audience:"business"|"guest"|"platform"="business"):Record<string,string> {
	const token=csrfToken(audience);
	return token?{"X-CSRF-Token":token}:{};
}
