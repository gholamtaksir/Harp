import { Power } from "lucide-react";
import { HarpGlyph } from "./Logo";
import { cn } from "./ui";

type Status = "disconnected" | "connecting" | "connected" | "disconnecting" | "error";

/** The large circular connect control on the dashboard. */
export function ConnectButton({
  status,
  onClick,
  disabled,
  warning,
}: {
  status: string;
  onClick: () => void;
  disabled?: boolean;
  /** Connected, but traffic does not get through. */
  warning?: boolean;
}) {
  const s = status as Status;
  const busy = s === "connecting" || s === "disconnecting";
  const connected = s === "connected";

  return (
    <div className="relative flex size-[196px] items-center justify-center">
      {/* Glow */}
      <div
        className={cn(
          "absolute inset-3 rounded-full blur-2xl transition-opacity duration-700",
          connected && !warning ? "animate-harp-pulse opacity-70" : "opacity-0",
        )}
        style={{ background: "conic-gradient(from 180deg, var(--accent), var(--cyan), var(--accent))" }}
      />
      {/* Ring */}
      <div
        className={cn("absolute inset-0 rounded-full p-[3px] transition-all duration-500", busy && "animate-harp-spin")}
        style={{
          background: connected
            ? warning
              ? "var(--warning)"
              : "conic-gradient(from 200deg, var(--accent), var(--cyan), var(--accent))"
            : busy
              ? "conic-gradient(from 0deg, transparent 0deg, var(--accent) 120deg, var(--cyan) 200deg, transparent 280deg)"
              : s === "error"
                ? "var(--danger)"
                : "var(--border-strong)",
        }}
      >
        <div className="size-full rounded-full bg-panel" />
      </div>

      <button
        type="button"
        onClick={onClick}
        disabled={disabled || busy}
        aria-label={connected ? "Disconnect" : "Connect"}
        className={cn(
          "group relative z-10 flex size-[164px] flex-col items-center justify-center gap-2 rounded-full border transition-all duration-300 active:scale-[0.97] disabled:cursor-default disabled:active:scale-100",
          connected
            ? "border-transparent text-white shadow-[0_20px_50px_-20px_var(--accent)]"
            : "border-line bg-panel-2 text-muted hover:border-line-strong hover:text-fg",
        )}
        style={
          connected
            ? { background: warning ? "linear-gradient(140deg, var(--warning), var(--accent))" : "linear-gradient(140deg, var(--accent), var(--cyan))" }
            : undefined
        }
      >
        {connected ? (
          <HarpGlyph className="size-14 drop-shadow" strokeWidth={8} />
        ) : (
          <Power className={cn("size-12 transition-transform", !busy && "group-hover:scale-105")} strokeWidth={1.8} />
        )}
        <span className="text-[11.5px] font-semibold tracking-[0.14em] uppercase">
          {connected ? "Connected" : busy ? (s === "connecting" ? "Connecting" : "Stopping") : "Connect"}
        </span>
      </button>
    </div>
  );
}
