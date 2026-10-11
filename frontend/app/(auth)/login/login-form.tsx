"use client";

import { useActionState } from "react";
import { login, type AuthFormState } from "@/app/actions/auth";
import styles from "../auth.module.css";

export function LoginForm() {
  const [state, formAction, pending] = useActionState<AuthFormState, FormData>(
    login,
    undefined,
  );

  return (
    <form action={formAction} className={styles.form} noValidate>
      <div className={styles.header}>
        <h1 className={styles.title}>Sign in</h1>
        <p className={styles.subtitle}>
          Continue where your architecture left off.
        </p>
      </div>

      {state?.message ? (
        <p className="alert alertError" role="alert">
          {state.message}
        </p>
      ) : null}

      <div className="field">
        <label className="label" htmlFor="email">
          Email
        </label>
        <input
          className="input"
          id="email"
          name="email"
          type="email"
          autoComplete="email"
          placeholder="you@archon.dev"
          required
          aria-invalid={Boolean(state?.errors?.email)}
        />
        {state?.errors?.email?.map((error) => (
          <p className="fieldError" key={error}>
            {error}
          </p>
        ))}
      </div>

      <div className="field">
        <label className="label" htmlFor="password">
          Password
        </label>
        <input
          className="input"
          id="password"
          name="password"
          type="password"
          autoComplete="current-password"
          placeholder="••••••••"
          required
          aria-invalid={Boolean(state?.errors?.password)}
        />
        {state?.errors?.password?.map((error) => (
          <p className="fieldError" key={error}>
            {error}
          </p>
        ))}
      </div>

      <button className="button buttonPrimary" type="submit" disabled={pending}>
        {pending ? "Signing in…" : "Sign in"}
      </button>

      <p className={styles.footer}>
        No account yet?{" "}
        <a className={styles.footerLink} href="/signup">
          Create one
        </a>
      </p>
    </form>
  );
}