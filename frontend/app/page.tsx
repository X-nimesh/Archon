import Link from "next/link";
import { Brand, Tagline } from "./brand";
import { getUser } from "@/lib/dal";
import styles from "./page.module.css";

export default async function Home() {
  // Decorative only — the page is public either way.
  const user = await getUser();

  return (
    <div className={styles.page}>
      <nav className={styles.nav}>
        <Brand />
        <div className={styles.navActions}>
          {user ? (
            <>
              <span className="hint">{user.email}</span>
              <Link className="button buttonPrimary" href="/dashboard">
                Open dashboard
              </Link>
            </>
          ) : (
            <>
              <Link className="button buttonGhost" href="/login">
                Sign in
              </Link>
              <Link className="button buttonPrimary" href="/signup">
                Get started
              </Link>
            </>
          )}
        </div>
      </nav>

      <main className={styles.hero}>
        <h1 className={styles.heroTitle}>
          From requirement to <span className={styles.heroAccent}>architecture</span>
        </h1>
        <p className={styles.heroLead}>
          Archon reads what you want to build, proposes the architecture, and
          generates the plan that gets you there.
        </p>
        <div className={styles.heroActions}>
          <Link className="button buttonPrimary" href={user ? "/dashboard" : "/signup"}>
            {user ? "Open dashboard" : "Create your account"}
          </Link>
          <Link className="button buttonGhost" href="/login">
            Sign in
          </Link>
        </div>
        <Tagline />
      </main>

      <section className={styles.features}>
        <article className={`card ${styles.feature}`}>
          <h2 className={styles.featureTitle}>Parse</h2>
          <p className={styles.featureBody}>
            Turn a written requirement into structured constraints and open
            questions.
          </p>
        </article>
        <article className={`card ${styles.feature}`}>
          <h2 className={styles.featureTitle}>Generate</h2>
          <p className={styles.featureBody}>
            Propose an architecture — services, data flows, and boundaries —
            grounded in your constraints.
          </p>
        </article>
        <article className={`card ${styles.feature}`}>
          <h2 className={styles.featureTitle}>Apply</h2>
          <p className={styles.featureBody}>
            Walk the plan with live agent activity until the change is on disk.
          </p>
        </article>
      </section>

      <footer className={styles.footer}>
        Archon — in the hush between stars, a name echoes.
      </footer>
    </div>
  );
}