"use client";
import React, { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import ModelHubTable from "@/components/AIHub/ModelHubTable";
import { useT } from "@/i18n";
function PublicModelHubContent() {
  const t = useT();
  const searchParams = useSearchParams()!;
  const key = searchParams.get("key");
  const [accessToken, setAccessToken] = useState<string | null>(null);
  useEffect(() => { if (key) setAccessToken(key); }, [key]);
  return <ModelHubTable accessToken={accessToken} publicPage={true} premiumUser={false} userRole={null} />;
}
export default function PublicModelHubPage() {
  const t = useT();
  return (<Suspense fallback={<div className="flex items-center justify-center min-h-screen">{t("Loading...")}</div>}><PublicModelHubContent /></Suspense>);
}
