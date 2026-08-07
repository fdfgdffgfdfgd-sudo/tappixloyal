"use client";
import { useEffect, useState } from "react";
import { Gift } from "lucide-react";
const base = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

export default function ReferralPage({ params }: { params: Promise<{ code: string }> }) {
  const [message, setMessage] = useState("Открываем приглашение…");
  useEffect(() => {
    params.then(({ code }) =>
      fetch(`${base}/public/referral/${code}`)
        .then((r) => r.json())
        .then((x) => {
          if (!x.success) throw new Error(x.error.message);
          sessionStorage.setItem("tappix_referral", code);
          location.replace(`/join/${x.data.token}`);
        })
        .catch((e) => setMessage(e.message)),
    );
  }, [params]);
  return <main className="guest-loading"><Gift /><strong>{message}</strong></main>;
}
