import Link from 'next/link';

/**
 * The Archon wordmark.
 *
 * Two forms:
 *  - with a version line, for the app shell sidebar
 *  - compact, for the auth screens and the landing page
 */
export function Brand({
  href = '/',
  version,
}: {
  href?: string;
  version?: string;
}) {
  return (
    <Link href={href} className="brand">
      <svg
        className="ic"
        width={20}
        height={20}
        style={{ color: 'var(--em)', strokeWidth: 1.6 }}
        aria-hidden="true"
        viewBox="0 0 16 16"
      >
        <path d="M8 1.5l5.8 3.3v6.4L8 14.5l-5.8-3.3V4.8z" />
        <path d="M8 5.2l2.6 1.5v2.6L8 10.8 5.4 9.3V6.7z" />
      </svg>
      <span>
        Archon
        {version ? <small>{version}</small> : null}
      </span>
    </Link>
  );
}

/** Tagline drawn from the project README. */
export function Tagline() {
  return (
    <p className="tagline">
      The doors are not locked.
      <br />
      They are waiting.
    </p>
  );
}