"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

interface UserInfo {
  sub: string;
  email?: string;
  name?: string;
  preferred_username?: string;
}

export default function Dashboard() {
  const router = useRouter();
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [apiResponse, setApiResponse] = useState<string | null>(null);

  useEffect(() => {
    fetch("/api/me", { credentials: "include" })
      .then((r) => {
        if (r.status === 401) {
          router.push("/");
          return null;
        }
        return r.json();
      })
      .then((data) => {
        if (data) setUser(data);
      })
      .catch(() => router.push("/"))
      .finally(() => setLoading(false));
  }, [router]);

  const callApi = async () => {
    try {
      const res = await fetch("/api/health", { credentials: "include" });
      const data = await res.json();
      setApiResponse(JSON.stringify(data, null, 2));
    } catch {
      setApiResponse("Error calling API");
    }
  };

  if (loading) {
    return (
      <main style={styles.container}>
        <div style={styles.spinner}>Loading...</div>
      </main>
    );
  }

  return (
    <main style={styles.container}>
      <div style={styles.layout}>
        <header style={styles.header}>
          <h1 style={styles.logo}>NexusGrid</h1>
          <div style={styles.userInfo}>
            <span style={styles.email}>
              {user?.email || user?.preferred_username || user?.sub}
            </span>
            <a href="/auth/logout" style={styles.logoutBtn}>
              Logout
            </a>
          </div>
        </header>

        <div style={styles.content}>
          <div style={styles.card}>
            <h2 style={styles.cardTitle}>Welcome back</h2>
            <p style={styles.cardText}>
              Logged in as:{" "}
              <strong>{user?.name || user?.preferred_username}</strong>
            </p>
            <p style={styles.cardText}>
              User ID: <code style={styles.code}>{user?.sub}</code>
            </p>
          </div>

          <div style={styles.card}>
            <h2 style={styles.cardTitle}>API Gateway Test</h2>
            <button onClick={callApi} style={styles.button}>
              Call /api/health
            </button>
            {apiResponse && (
              <pre style={styles.pre}>{apiResponse}</pre>
            )}
          </div>

          <div style={styles.card}>
            <h2 style={styles.cardTitle}>Services</h2>
            <div style={styles.serviceGrid}>
              {[
                { name: "Auth Service", status: "running", port: "8080" },
                { name: "API Gateway", status: "running", port: "8080" },
                { name: "Keycloak", status: "running", port: "8080" },
                { name: "RabbitMQ", status: "running", port: "5672" },
              ].map((svc) => (
                <div key={svc.name} style={styles.serviceItem}>
                  <span style={styles.statusDot} />
                  <div>
                    <div style={styles.serviceName}>{svc.name}</div>
                    <div style={styles.servicePort}>:{svc.port}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    minHeight: "100vh",
    background: "#0f1117",
  },
  layout: {
    maxWidth: "1100px",
    margin: "0 auto",
    padding: "1rem",
  },
  header: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    padding: "1rem 0",
    borderBottom: "1px solid #2d3148",
    marginBottom: "2rem",
  },
  logo: {
    fontSize: "1.5rem",
    fontWeight: 700,
    background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
    WebkitBackgroundClip: "text",
    WebkitTextFillColor: "transparent",
  },
  userInfo: {
    display: "flex",
    alignItems: "center",
    gap: "1rem",
  },
  email: {
    color: "#94a3b8",
    fontSize: "0.9rem",
  },
  logoutBtn: {
    background: "#2d3148",
    color: "#e2e8f0",
    padding: "0.4rem 1rem",
    borderRadius: "6px",
    fontSize: "0.9rem",
    cursor: "pointer",
  },
  content: {
    display: "grid",
    gap: "1.5rem",
    gridTemplateColumns: "repeat(auto-fit, minmax(300px, 1fr))",
  },
  card: {
    background: "#1a1d27",
    border: "1px solid #2d3148",
    borderRadius: "10px",
    padding: "1.5rem",
  },
  cardTitle: {
    fontSize: "1.1rem",
    fontWeight: 600,
    marginBottom: "1rem",
    color: "#e2e8f0",
  },
  cardText: {
    color: "#94a3b8",
    marginBottom: "0.5rem",
    fontSize: "0.9rem",
  },
  code: {
    background: "#0f1117",
    padding: "0.1rem 0.4rem",
    borderRadius: "4px",
    fontSize: "0.8rem",
    fontFamily: "monospace",
    color: "#8b5cf6",
  },
  button: {
    background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
    color: "#fff",
    border: "none",
    padding: "0.6rem 1.2rem",
    borderRadius: "6px",
    cursor: "pointer",
    fontWeight: 600,
    fontSize: "0.9rem",
    marginBottom: "1rem",
  },
  pre: {
    background: "#0f1117",
    padding: "1rem",
    borderRadius: "6px",
    fontSize: "0.8rem",
    color: "#a5f3fc",
    fontFamily: "monospace",
    overflow: "auto",
    marginTop: "0.5rem",
  },
  serviceGrid: {
    display: "flex",
    flexDirection: "column",
    gap: "0.75rem",
  },
  serviceItem: {
    display: "flex",
    alignItems: "center",
    gap: "0.75rem",
  },
  statusDot: {
    width: "8px",
    height: "8px",
    borderRadius: "50%",
    background: "#22c55e",
    flexShrink: 0,
  },
  serviceName: {
    fontSize: "0.9rem",
    color: "#e2e8f0",
  },
  servicePort: {
    fontSize: "0.75rem",
    color: "#64748b",
    fontFamily: "monospace",
  },
  spinner: {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    minHeight: "100vh",
    color: "#94a3b8",
  },
};
