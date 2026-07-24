import Link from "next/link";

// 404 (S-15). "Page not found" + link back to Overview.
export default function NotFound() {
  return (
    <section aria-labelledby="notfound-heading">
      <h1 id="notfound-heading" style={{ fontSize: "var(--text-h1)", fontWeight: 600 }}>
        Page not found
      </h1>
      <p style={{ color: "var(--color-text-secondary)", marginTop: "var(--space-sm)" }}>
        The page you&apos;re looking for doesn&apos;t exist. <Link href="/">Return to Overview</Link>.
      </p>
    </section>
  );
}
