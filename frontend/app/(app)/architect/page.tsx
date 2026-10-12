import type { Metadata } from 'next';
import { Icon } from '@/components/Icon';
import { verifySession } from '@/lib/dal';
import { D2RenderError, renderD2 } from '@/lib/d2';
import styles from './architect.module.css';

export const metadata: Metadata = {
  title: 'Architect',
};

/* ------------------------------------------------------------ demo document */

/**
 * PLACEHOLDER — delete this constant when the architecture endpoint lands.
 *
 * The Architect page is supposed to render the D2 document returned by the
 * backend (a design proposed by the Architecture agent, stored per project).
 * That endpoint does not exist yet, so there is nothing to fetch. Until it does,
 * this short local document stands in: it describes the shape Archon is
 * actually built in, and it is labelled as provisional everywhere it appears so
 * nobody mistakes it for a real proposal.
 *
 * The replacement is a single call to the API and the delete of this block —
 * `renderD2(source)` takes exactly this shape.
 */
const PLACEHOLDER_D2 = `direction: right

edge: Edge {
  operators: Operators
  browser: Browser
  operators -> browser
}

platform: Archon {
  web: Web
  guard: proxy.ts
  api: API
  db: Postgres
  web -> guard: every request
  guard -> api: /auth/*
  api -> db: pgx
}

control: Control plane {
  arch: Architecture
  plan: Plan
  verify: Verify
  arch -> plan: topology
  plan -> verify
}

edge -> platform: HTTPS
control -> platform: proposed, not applied {style.stroke-dash: 3}
`;

/** Spoken description of the diagram, for anyone who cannot see it. */
const DIAGRAM_LABEL =
  'Proposed topology, in three tiers. At the edge, operators reach a browser over HTTPS. The Archon tier holds the request path: a web app, an optimistic route guard in proxy.ts, a Go API, and a PostgreSQL database reached over a pgx pool. Alongside sits the control plane, where the Architecture agent produces a topology, the Plan agent turns that into a plan, and Verification checks the applied state. A dashed line from the control plane back to Archon marks the topology as proposed and not applied.';

/* ------------------------------------------------------------------- page */

export default async function ArchitectPage() {
  // Authoritative guard — `proxy.ts` only checks that the cookie exists, so a
  // revoked session is caught here, not just on first load.
  const { user } = await verifySession();

  // A broken diagram is a content problem, not a server failure: the rest of
  // the page is still worth showing, so the failure is rendered in place.
  let svg: string | null = null;
  let failure: { message: string; detail: string } | null = null;

  try {
    svg = await renderD2(PLACEHOLDER_D2, { theme: 'dark' });
  } catch (error) {
    failure =
      error instanceof D2RenderError
        ? { message: error.message, detail: error.detail }
        : { message: 'The diagram could not be rendered.', detail: '' };
  }

  return (
    <div className="page">
      <div className="ph">
        <div>
          <nav className={styles.crumb} aria-label="Breadcrumb">
            <span>Projects</span>
            <Icon name="arrow" className={styles.crumbSep} />
            <span aria-current="page">Architect</span>
          </nav>

          <h1>Architect</h1>
          <p>
            The topology below is a proposal, not a deployment. Nothing here has been planned or
            applied — it is the shape the Architecture agent would propose for a project, and it
            only becomes real once a plan is reviewed and an apply is run.
          </p>
        </div>
      </div>

      <section className="pn">
        <div className="pn-h">
          <h2>Proposed topology</h2>
          <span className="cap">provisional preview</span>
        </div>

        {svg === null ? (
          <div className={styles.fail}>
            <b>The topology could not be drawn</b>
            {failure?.message ?? 'The diagram renderer returned nothing.'}
            {failure?.detail ? (
              <code className={`mono ${styles.failCode}`}>{failure.detail}</code>
            ) : null}
          </div>
        ) : (
          <>
            <div className={styles.canvas}>
              <div
                className={styles.diagram}
                role="img"
                aria-label={DIAGRAM_LABEL}
                dangerouslySetInnerHTML={{ __html: svg }}
              />
            </div>

            {/* Visible, and not just in a comment: this is a stand-in. */}
            <p className={styles.note}>
              <Icon name="warn" />
              <span>
                Placeholder document. There is no architecture endpoint yet, so this diagram is
                compiled from a bundled example. It will be replaced by the Architecture agent’s
                stored proposal as soon as the API can serve one.
              </span>
            </p>

            <details className={styles.src}>
              <summary>
                <Icon name="code" />
                Diagram source (D2)
              </summary>
              <pre className={`mono ${styles.srcCode}`} tabIndex={0}>
                {PLACEHOLDER_D2}
              </pre>
            </details>
          </>
        )}
      </section>

      <p className={`sub ${styles.foot}`}>
        {`Signed in as ${user.name} (${user.email}).`}
      </p>
    </div>
  );
}