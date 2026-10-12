import { verifySession } from '@/lib/dal';
import { AppShell } from './shell';

/**
 * Authenticated app shell: collapsible rail + sticky top bar.
 *
 * The guard lives here rather than in each page. Note this is a *layout*, and
 * layouts do not re-run on client-side navigation — `verifySession()` is still
 * called by each protected page so a revoked session is caught on every route
 * change, not just the first load.
 */
export default async function AppLayout({ children }: LayoutProps<'/'>) {
  const { user } = await verifySession();

  return (
    <AppShell user={{ name: user.name, email: user.email }}>
      {children}
    </AppShell>
  );
}