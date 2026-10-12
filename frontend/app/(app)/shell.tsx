'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useState } from 'react';
import { Icon } from '@/components/Icon';
import { logout } from '@/app/actions/auth';
import styles from './shell.module.css';

/** Per-project sections. Disabled until those views exist. */
const PROJECT_SECTIONS = [
  { label: 'Overview', icon: 'dash', href: '/dashboard', enabled: true },
  { label: 'Architect', icon: 'graph', href: '/architect', enabled: true },
  { label: 'Infrastructure', icon: 'code', href: null, enabled: false },
  { label: 'Approvals', icon: 'check', href: null, enabled: false },
  { label: 'Deployments', icon: 'up', href: null, enabled: false },
  { label: 'Agent activity', icon: 'clock', href: null, enabled: false },
] as const;

export function AppShell({
  user,
  children,
}: {
  user: { name: string; email: string };
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const [rail, setRail] = useState(false);
  const [open, setOpen] = useState(false);

  const isCurrent = (href: string | null) =>
    href !== null && pathname === href;

  return (
    <div
      className={`${styles.app} ${rail ? styles.rail : ''} ${open ? styles.open : ''}`}
    >
      <aside className={styles.side} aria-label="Primary">
        <div className={styles.brandRow}>
          <Icon name="graph" className={styles.brandIcon} />
          <div>
            <b>Archon</b>
            <small>v0.4.2-alpha</small>
          </div>
        </div>

        <nav className={styles.nav} aria-label="Sections">
          <div className={styles.navH}>Workspace</div>

          <Link
            href="/dashboard"
            className={`${styles.navI} ${isCurrent('/dashboard') ? styles.current : ''}`}
            aria-current={isCurrent('/dashboard') ? 'page' : undefined}
            onClick={() => setOpen(false)}
          >
            <Icon name="dash" />
            <span className={styles.lbl}>Overview</span>
          </Link>

          <Link
            href="/projects/new"
            className={`${styles.navI} ${isCurrent('/projects/new') ? styles.current : ''}`}
            aria-current={isCurrent('/projects/new') ? 'page' : undefined}
            onClick={() => setOpen(false)}
          >
            <Icon name="plus" />
            <span className={styles.lbl}>New project</span>
          </Link>

          <div className={styles.navH}>Project</div>

          {PROJECT_SECTIONS.map((section) =>
            section.enabled ? (
              <Link
                key={section.label}
                href={section.href!}
                className={`${styles.navI} ${isCurrent(section.href) ? styles.current : ''}`}
                aria-current={isCurrent(section.href) ? 'page' : undefined}
                onClick={() => setOpen(false)}
              >
                <Icon name={section.icon} />
                <span className={styles.lbl}>{section.label}</span>
              </Link>
            ) : (
              <span
                key={section.label}
                className={styles.navI}
                aria-disabled="true"
                title="Included in the full UI kit"
              >
                <Icon name={section.icon} />
                <span className={styles.lbl}>{section.label}</span>
              </span>
            ),
          )}
        </nav>
      </aside>

      <button
        className={styles.scrim}
        onClick={() => setOpen(false)}
        aria-label="Close navigation"
        tabIndex={open ? 0 : -1}
      />

      <div className={styles.main}>
        <header className={styles.top}>
          <button
            className={`icon-btn ${styles.onlyNarrow}`}
            onClick={() => setOpen(true)}
            aria-label="Open navigation"
          >
            <Icon name="menu" />
          </button>

          <button
            className={`icon-btn ${styles.onlyWide}`}
            onClick={() => setRail((r) => !r)}
            aria-label={rail ? 'Expand sidebar' : 'Collapse sidebar'}
            aria-expanded={!rail}
          >
            <Icon name="panel" />
          </button>

          <div className={styles.crumb}>
            <span className="bd ne">{process.env.NEXT_PUBLIC_WORKSPACE_NAME ?? 'Workspace'}</span>
          </div>

          <div className={styles.spacer} />

          <div className={styles.user}>
            <div className={styles.avatar} aria-hidden="true">
              {user.name
                .split(/\s+/)
                .slice(0, 2)
                .map((p) => p[0])
                .join('')}
            </div>
            <div className={styles.userMeta}>
              <span className={styles.userName}>{user.name}</span>
              <span className={styles.userEmail}>{user.email}</span>
            </div>
            <form action={logout}>
              <button className={`btn sm`} type="submit">
                Sign out
              </button>
            </form>
          </div>
        </header>

        <div className={styles.scroll}>{children}</div>
      </div>
    </div>
  );
}