"use client";

import { useActionState } from "react";
import { signup, type AuthFormState } from "@/app/actions/auth";
import styles from "../auth.module.css";

export function SignupForm() {
  const [state, formAction, pending] = useActionState<AuthFormState, FormData>(
    signup,
    undefined,
  );

  return (
    <form action={formAction} className={styles.form} noValidate>
      <div className={styles.header}>
        <h1 className={styles.title}>Create your account</h1>
        <p className={styles.subtitle}>
          Start turning requirements into architecture.
        </p>
      </div>

      {state?.message ? (
        <p className="alert alertError" role="alert">
          {state.message}
        </p>
      ) : null}

      <div className="field">
        <label className="label" htmlFor="name">
          Name
        </label>
        <input
          className="input"
          id="name"
          name="name"
          type="text"
          autoComplete="name"
          placeholder="Nimesh"
          required
          aria-invalid={Boolean(state?.errors?.name)}
        />
        {state?.errors?.name?.map((error) => (
          <p className="fieldError" key={error}>
            {error}
          </p>
        ))}
      </div>

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
          autoComplete="new-password"
          placeholder="At least 8 characters"
          required
          aria-invalid={Boolean(state?.errors?.password)}
        />
        {state?.errors?.password?.map((error) => (
          <p className="fieldError" key={error}>
            {error}
          </p>
        ))}
        <p className="hint">Minimum 8 characters.</p>
      </div>

      <button className="button buttonPrimary" type="submit" disabled={pending}>
        {pending ? "Creating account…" : "Create account"}
      </button>

      <p className={styles.footer}>
        Already have an account?{" "}
        <a className={styles.footerLink} href="/login">
          Sign in
        </a>
      </p>
    </form>
  );
}