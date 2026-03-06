export default function Home() {
  return (
    <main style={styles.container}>
      <div style={styles.card}>
        <h1 style={styles.title}>NexusGrid</h1>
        <p style={styles.subtitle}>Microservices platform</p>
        <a href="/auth/login" style={styles.button}>
          Sign in with Keycloak
        </a>
      </div>
    </main>
  );
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    minHeight: "100vh",
    padding: "1rem",
  },
  card: {
    background: "#1a1d27",
    border: "1px solid #2d3148",
    borderRadius: "12px",
    padding: "3rem",
    textAlign: "center",
    maxWidth: "400px",
    width: "100%",
  },
  title: {
    fontSize: "2.5rem",
    fontWeight: 700,
    background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
    WebkitBackgroundClip: "text",
    WebkitTextFillColor: "transparent",
    marginBottom: "0.5rem",
  },
  subtitle: {
    color: "#94a3b8",
    marginBottom: "2rem",
    fontSize: "1rem",
  },
  button: {
    display: "inline-block",
    background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
    color: "#fff",
    padding: "0.8rem 2rem",
    borderRadius: "8px",
    fontWeight: 600,
    fontSize: "1rem",
    cursor: "pointer",
    transition: "opacity 0.2s",
  },
};
