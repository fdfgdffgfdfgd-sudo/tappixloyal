"use client";
import { useEffect, useState } from "react";
import { Gift } from "lucide-react";
import { API_URL as base } from "@/lib/api";

export default function ReferralPage({ params }: { params: Promise<{ code: string }> }) {
  const [message, setMessage] = useState("Открываем приглашение…");
  useEffect(() => {
    params.then(({ code }) => {
      const stored=sessionStorage.getItem("tappix_referral_click_id");
      const anonymousId=stored||crypto.randomUUID();
      if(!stored)sessionStorage.setItem("tappix_referral_click_id",anonymousId);
      return fetch(`${base}/public/referral/${code}?anonymousId=${encodeURIComponent(anonymousId)}`)
        .then((r) => r.json())
        .then((x) => {
          if (!x.success) throw new Error(x.error.message);
          sessionStorage.setItem("tappix_referral", code);
          location.replace(`/join/${x.data.token}`);
        })
        .catch((e) => setMessage(e.message));
    });
  }, [params]);
  return <main className="guest-loading"><Gift /><strong>{message}</strong></main>;
}
