"use client";

// Top-level fallback boundary (Next requires its own <html>/<body>). Used only
// if the root layout itself throws; route errors use app/error.tsx.
export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="en">
      <body style={{ fontFamily: "system-ui, sans-serif", padding: 24 }}>
        <main role="alert">
          <h1>Something went wrong</h1>
          <p>The application encountered an unexpected error.</p>
          <button type="button" onClick={reset}>
            Retry
          </button>
        </main>
      </body>
    </html>
  );
}
