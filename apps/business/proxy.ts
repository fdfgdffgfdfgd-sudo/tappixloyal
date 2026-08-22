import { NextRequest, NextResponse } from "next/server";

const PUBLIC_PREFIXES = ["/welcome", "/login", "/join/", "/r/", "/site/", "/customer"];

function loginRedirect(request: NextRequest) {
  const url = new URL("/login", request.url);
  url.searchParams.set("next", `${request.nextUrl.pathname}${request.nextUrl.search}`);
  return NextResponse.redirect(url);
}

export function proxy(request: NextRequest) {
  const path = request.nextUrl.pathname;
  if (PUBLIC_PREFIXES.some((prefix) => path === prefix || path.startsWith(prefix))) {
    return NextResponse.next();
  }
  if (path === "/admin") {
    return request.cookies.has("tappix_platform_access") || request.cookies.has("tappix_platform_csrf") ? NextResponse.next() : loginRedirect(request);
  }
  return request.cookies.has("tappix_access") || request.cookies.has("tappix_csrf") ? NextResponse.next() : loginRedirect(request);
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|icon.png|tappix-mark.png|robots.txt).*)"],
};
