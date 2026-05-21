"use client";

import { useSearchParams } from "next/navigation";

import { LoginPage } from "@/features/auth/components/login-page";

function normalizeNextPath(value: string | null): string {
  if (!value || !value.startsWith("/") || value.startsWith("//")) {
    return "";
  }
  return value;
}

export function LoginRoute() {
  const searchParams = useSearchParams();
  return <LoginPage nextPath={normalizeNextPath(searchParams.get("next"))} />;
}
