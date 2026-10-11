import type { Metadata } from "next";
import { logout } from "@/app/actions/auth";
import { Brand } from "@/app/brand";
import { verifySession } from "@/lib/dal";
import styles from "./dashboard.module.css";

export const metadata: Metadata = {
  title: "Dashboard",
};

function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0])
    .join("");
}

const UPCOMING = [
  {
    title: "Parse a requirement",
    body: "Describe what you want to build and Archon will pull out the constraints.",
  },
  {
    title: "Generate an architecture",
    body: "Review proposed services, boundaries, and data flows before anything is written.",
  },
  {
    title: "Apply the plan",
    body: "Walk the change with live agent activity until it lands on disk.",
  },
];

export default async function DashboardPage() {
  // Authoritative guard — redirects to /login unless the cookie is backed by
  // a live account. `proxy.ts` only checks that the cookie exists.
  const { user } = await verifySession();

  return (
    <div className={styles.page}>
      <nav className={styles.nav}>
        <Brand />
        <div className={styles.navUser}>
          <div className={styles.avatar} aria-hidden="true">
            {initials(user.name)}
          </div>
          <div>
            <div className={styles.navName}>{user.name}</div>
            <div className={styles.navEmail}>{user.email}</div>
          </div>
          {/* logout is a Server Action — usable directly as a form action. */}
          <form action={logout}>
            <button className="button buttonGhost" type="submit">
              Sign out
            </button>
          </form>
        </div>
      </nav>

      <main className={styles.main}>
        <h1 className={styles.heading}>Welcome back, {user.name.split(" ")[0]}</h1>
        <p className={styles.lead}>
          You are signed in. Your workspace is ready.
        </p>

        <section className={styles.stats}>
          <div className={`card ${styles.stat}`}>
            <div className={styles.statLabel}>Requirements</div>
            <div className={`${styles.statValue} ${styles.statValueEmpty}`}>
              None yet
            </div>
          </div>
          <div className={`card ${styles.stat}`}>
            <div className={styles.statLabel}>Plans</div>
            <div className={`${styles.statValue} ${styles.statValueEmpty}`}>
              None yet
            </div>
          </div>
          <div className={`card ${styles.stat}`}>
            <div className={styles.statLabel}>Member since</div>
            <div className={styles.statValue}>
              {new Date(user.createdAt).toLocaleDateString("en-US", {
                month: "short",
                day: "numeric",
                year: "numeric",
              })}
            </div>
          </div>
        </section>

        <section className={`card ${styles.panel}`}>
          <h2 className={styles.panelTitle}>What Archon does next</h2>
          <p className={styles.panelBody}>
            These three stages are what Archon is being built around. They are
            not wired up yet.
          </p>
          <div className={styles.steps}>
            {UPCOMING.map((step, index) => (
              <div className={styles.step} key={step.title}>
                <span className={styles.stepIndex} aria-hidden="true">
                  {index + 1}
                </span>
                <div>
                  <div className={styles.stepTitle}>{step.title}</div>
                  <div className={styles.stepBody}>{step.body}</div>
                </div>
              </div>
            ))}
          </div>
        </section>
      </main>
    </div>
  );
}