import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getUser } from "@/lib/dal";
import { LoginForm } from "./login-form";

export const metadata: Metadata = {
  title: "Sign in",
};

export default async function LoginPage() {
  // Already signed in? Skip the form. `proxy.ts` handles the cheap case; this
  // catches a stale or revoked cookie.
  const user = await getUser();
  if (user) redirect("/dashboard");

  return <LoginForm />;
}