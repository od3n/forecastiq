import type { ReactNode } from "react";
import styles from "./auth.module.css";

// S-08 auth section (public). Centered card container for the SDK-backed forms.
export default function AuthLayout({ children }: { children: ReactNode }) {
  return <div className={styles.container}>{children}</div>;
}
