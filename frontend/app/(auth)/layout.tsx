import { Brand } from "@/app/brand";
import styles from "./auth.module.css";

/**
 * Centered card shell shared by /login and /signup.
 *
 * Does no auth checking on purpose — per the Next.js auth guide, layouts do not
 * re-run on client-side navigation, so guarding here would let an already
 * rendered page keep its content. The real check lives in each page.
 */
export default function AuthLayout({ children }: LayoutProps<"/">) {
  return (
    <div className={styles.shell}>
      <div className={`card ${styles.card}`}>
        <div className={styles.header}>
          <Brand />
        </div>
        {children}
      </div>
    </div>
  );
}