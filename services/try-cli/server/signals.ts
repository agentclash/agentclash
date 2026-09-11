/** Shared by the production entrypoint and the isolated process rehearsal. */
export function installSignals(app: { drain(): void; close(): Promise<void> }) {
  let stopping = false;
  const stop = () => {
    if (stopping) return;
    stopping = true;
    void app.close().then(() => process.exit(0), () => {
      console.error("[try-cli] shutdown cleanup incomplete"); process.exit(1);
    });
  };
  process.on("SIGUSR2", () => { app.drain(); console.log("[try-cli] admission drained"); });
  process.on("SIGTERM", stop);
  process.on("SIGINT", stop);
  return { get stopping() { return stopping; } };
}
