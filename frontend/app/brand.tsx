import Link from "next/link";
import styles from "./brand.module.css";

/** The Archon wordmark. Used in the navbar and on the auth screens. */
export function Brand({ href = "/" }: { href?: string }) {
  return (
    <Link href={href} className="brand">
      <span className="brandMark" aria-hidden="true">
        ✦
      </span>
      Archon
    </Link>
  );
}

/** Tagline drawn from the project README. */
export function Tagline() {
  return (
    <p className={styles.tagline}>
      The doors are not locked.
      <br />
      They are waiting.
    </p>
  );
}