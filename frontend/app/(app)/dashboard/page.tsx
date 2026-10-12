import type { Metadata } from 'next';
import Link from 'next/link';
import { Icon } from '@/components/Icon';
import { verifySession } from '@/lib/dal';
import styles from './dashboard.module.css';

export const metadata: Metadata = {
  title: 'Overview',
};

export default async function DashboardPage() {
  // Authoritative guard — `proxy.ts` only checks that the cookie exists, so a
  // revoked session is caught here on every route change, not just first load.
  const { user } = await verifySession();

  return (
    <div className="page">
      <div className="ph">
        <div>
          <h1>Overview</h1>
          <p>
            {`Signed in as ${user.name} (${user.email}). Project health, guardrails,
            resources and agent activity appear here once you create a project.`}
          </p>
        </div>

        <div className="acts">
          <Link className="btn p" href="/projects/new">
            <Icon name="plus" />
            New project
          </Link>
        </div>
      </div>

      <section className="pn">
        <div className="pn-h">
          <h2>Projects</h2>
        </div>

        <div className="pn-b">
          <div className={`empty ${styles.empty}`}>
            <b>No projects yet</b>
            Create a project to see its health, guardrails, resources and agent
            activity here.
          </div>

          <div className={styles.cta}>
            <Link className="btn p" href="/projects/new">
              <Icon name="plus" />
              New project
            </Link>
          </div>
        </div>
      </section>
    </div>
  );
}
