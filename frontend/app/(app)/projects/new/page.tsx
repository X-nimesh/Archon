import type { Metadata } from 'next';
import Link from 'next/link';
import { Icon } from '@/components/Icon';
import { verifySession } from '@/lib/dal';
import { NewProjectWizard } from './wizard';
import styles from './new-project.module.css';

export const metadata: Metadata = {
  title: 'New project',
};

export default async function NewProjectPage() {
  // Authoritative guard — `proxy.ts` only checks that a cookie exists.
  await verifySession();

  return (
    <div className="page">
      <nav className={styles.crumb} aria-label="Breadcrumb">
        <Link href="/dashboard">
          <Icon name="dash" />
          Overview
        </Link>
        <span aria-hidden="true">/</span>
        <span aria-current="page">New project</span>
      </nav>

      <div className="ph">
        <div>
          <h1>New project</h1>
          <p>
            Tell Archon what you are building and what limits apply. Nothing is
            created in AWS until a human approves a plan.
          </p>
        </div>
      </div>

      <NewProjectWizard />
    </div>
  );
}