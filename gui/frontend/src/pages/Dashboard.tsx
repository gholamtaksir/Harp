import {
  ArrowDown,
  ArrowRight,
  ArrowUp,
  Cable,
  Clock,
  Gauge,
  Globe2,
  MapPin,
  MonitorSmartphone,
  Network,
  Radar,
  RefreshCw,
  Server,
  ShieldAlert,
  ShieldCheck,
  SkipForward,
  UserPlus,
  Zap,
} from "lucide-react";
import { useState, type ReactNode } from "react";
import { ConnectButton } from "../components/ConnectButton";
import { Logo } from "../components/Logo";
import { TrafficChart } from "../components/TrafficChart";
import { Badge, Button, Callout, Card, CardHeader, CopyButton, Kbd, Segmented, Spinner, useFeedback } from "../components/ui";
import { api, errorMessage } from "../lib/api";
import { countryName, formatBytes, formatDuration, formatSpeed } from "../lib/format";
import { useNow, useStore } from "../lib/store";

export function Dashboard() {
  const { state, stats, info, setPage, draft } = useStore();
  const { toast } = useFeedback();
  const [switching, setSwitching] = useState(false);
  const now = useNow(1000);

  const connected = state.status === "connected";
  const busy = state.status === "connecting" || state.status === "disconnecting";
  const degraded = connected && state.trace.checkedAt > 0 && !state.trace.ok;
  const latest = stats[stats.length - 1];

  const toggle = async () => {
    try {
      if (connected) await api.Disconnect();
      else await api.Connect();
    } catch (error) {
      const message = errorMessage(error);
      if (!message.includes("administrator") && !message.includes("no WARP account")) {
        toast("danger", "Could not connect", message);
      }
    }
  };

  const changeMode = async (mode: string) => {
    if (mode === state.mode) return;
    setSwitching(true);
    try {
      await api.SetMode(mode);
    } catch (error) {
      toast("danger", "Could not switch mode", errorMessage(error));
    } finally {
      setSwitching(false);
    }
  };

  if (!state.hasAccount) return <Welcome onSetup={() => setPage("account")} />;

  const enabledListeners = (draft?.proxy.listeners ?? []).filter((l) => l.enabled);
  const listenerUrls = connected
    ? (state.listeners ?? [])
    : state.mode === "proxy" || draft?.tun.serveProxy
      ? enabledListeners.map((l) => `${l.type === "socks" ? "socks5" : l.type}://${l.address}:${l.port}`)
      : [];

  return (
    <div className="space-y-5">
      <div className="grid grid-cols-[minmax(0,1fr)_340px] gap-5">
        <Card className="relative overflow-hidden">
          <div
            className="pointer-events-none absolute -top-40 -right-40 size-96 rounded-full opacity-[0.08] blur-3xl transition-opacity duration-700"
            style={{ background: "radial-gradient(circle, var(--accent), transparent 70%)", opacity: connected ? 0.18 : 0.06 }}
          />
          <div className="relative flex items-center gap-10 px-8 py-8">
            <ConnectButton status={state.status} onClick={toggle} disabled={switching} warning={degraded} />
            <div className="min-w-0 flex-1">
              <StatusHeadline />
              <div className="mt-6 flex flex-wrap items-center gap-3">
                <Segmented
                  value={state.mode}
                  onChange={changeMode}
                  options={[
                    { value: "proxy", label: "Proxy", icon: <Cable className="size-3.5" />, disabled: busy || switching },
                    { value: "tun", label: "TUN", icon: <MonitorSmartphone className="size-3.5" />, disabled: busy || switching },
                  ]}
                />
                {switching && <Spinner />}
                <span className="text-[12px] text-faint">
                  {state.mode === "tun" ? "All apps on this device use WARP" : "Only apps using the proxy use WARP"}
                </span>
              </div>
              {listenerUrls.length > 0 && (
                <div className="mt-4 flex flex-wrap gap-2">
                  {listenerUrls.map((url) => (
                    <div key={url} className="flex h-8 items-center gap-1 rounded-lg border border-line bg-panel-2 pr-0.5 pl-2.5">
                      <span className="selectable font-mono text-[12px] text-muted">{url}</span>
                      <CopyButton text={url} label="Copy address" />
                    </div>
                  ))}
                </div>
              )}
              <NoticeBanner />
              <ErrorBanner />
            </div>
          </div>
        </Card>

        <WarpStatus />
      </div>

      <div className="grid grid-cols-4 gap-5">
        <Metric icon={<Clock className="size-4" />} label="Session" value={connected ? formatDuration(now - state.connectedAt) : "—"} />
        <Metric
          icon={<ArrowDown className="size-4" />}
          label="Downloaded"
          value={connected ? formatBytes(latest?.downTotal ?? 0) : "—"}
          hint={connected ? formatSpeed(latest?.down ?? 0) : undefined}
          tone="accent"
        />
        <Metric
          icon={<ArrowUp className="size-4" />}
          label="Uploaded"
          value={connected ? formatBytes(latest?.upTotal ?? 0) : "—"}
          hint={connected ? formatSpeed(latest?.up ?? 0) : undefined}
          tone="cyan"
        />
        <Metric
          icon={<Network className="size-4" />}
          label="Active connections"
          value={connected ? String(latest?.connections ?? 0) : "—"}
          action={
            connected ? (
              <button className="text-[12px] font-medium text-accent hover:underline" onClick={() => setPage("connections")}>
                View
              </button>
            ) : undefined
          }
        />
      </div>

      <Card>
        <CardHeader
          title="Traffic"
          description="Throughput over the last two minutes"
          actions={
            <div className="flex items-center gap-5 text-[12px]">
              <Legend color="bg-accent" label="Download" value={formatSpeed(connected ? (latest?.down ?? 0) : 0)} />
              <Legend color="bg-cyan" label="Upload" value={formatSpeed(connected ? (latest?.up ?? 0) : 0)} />
            </div>
          }
        />
        <div className="px-3 pb-3">
          {connected || stats.length > 0 ? (
            <TrafficChart stats={connected ? stats : []} />
          ) : (
            <div className="flex h-[210px] items-center justify-center text-[13px] text-faint">Connect to see live traffic</div>
          )}
        </div>
      </Card>

      {info && info.os === "windows" && state.mode === "tun" && !info.elevated && state.status === "disconnected" && (
        <Callout
          tone="warning"
          title="TUN mode needs administrator rights"
          action={
            <Button size="sm" variant="primary" onClick={() => api.RelaunchAsAdmin(true)}>
              Restart as administrator
            </Button>
          }
        >
          Harp will restart with elevated rights and connect automatically.
        </Callout>
      )}
    </div>
  );
}

function StatusHeadline() {
  const { state } = useStore();
  const degraded = state.status === "connected" && state.trace.checkedAt > 0 && !state.trace.ok;
  const titles: Record<string, string> = {
    connected: "You're protected",
    connecting: "Connecting to WARP…",
    disconnecting: "Disconnecting…",
    error: "Connection failed",
    disconnected: "You're not protected",
  };
  const subtitle = degraded
    ? state.automatic
      ? "No traffic gets through this endpoint. Harp is trying another one."
      : "No traffic gets through this endpoint. It is probably blocked on this network."
    : state.status === "connected"
      ? state.mode === "tun"
        ? "All traffic on this device is routed by your rules."
        : "Apps configured to use the proxy are routed by your rules."
      : state.status === "error"
        ? "Harp could not start the connection."
        : state.status === "disconnected"
          ? "Press connect to route your traffic through Cloudflare WARP."
          : "This usually takes a moment.";
  return (
    <div>
      <div className="flex items-center gap-2.5">
        <h2 className="text-[24px] font-semibold tracking-tight">
          {degraded ? "WARP isn't responding" : (titles[state.status] ?? state.status)}
        </h2>
        {state.status === "connected" && state.trace.warp === "plus" && (
          <Badge tone="accent">
            WARP+
          </Badge>
        )}
      </div>
      <p className="mt-1 text-[13px] text-muted">{subtitle}</p>
    </div>
  );
}

function NoticeBanner() {
  const { state } = useStore();
  if (!state.notice || (state.status !== "connecting" && state.status !== "connected")) return null;
  return (
    <Callout tone="warning" className="mt-5">
      {state.notice}
    </Callout>
  );
}

function ErrorBanner() {
  const { state, info, setPage } = useStore();
  if (state.status !== "error" || !state.error) return null;
  if (state.errorCode === "no_endpoint") {
    return (
      <Callout
        tone="danger"
        className="mt-5"
        stacked
        title="No WARP endpoint answers"
        action={
          <Button size="sm" variant="primary" icon={<Radar className="size-3.5" />} onClick={() => setPage("endpoints")}>
            Scan for endpoints
          </Button>
        }
      >
        Harp tried the known endpoints and a quick scan, and none answered. Your network may block WireGuard; a thorough scan
        tries many more addresses and ports.
      </Callout>
    );
  }
  if (state.errorCode === "elevation_required") {
    return (
      <Callout
        tone="warning"
        className="mt-5"
        stacked
        title="Administrator rights required"
        action={
          info?.canRelaunch ? (
            <Button size="sm" variant="primary" onClick={() => api.RelaunchAsAdmin(true)}>
              Restart as administrator
            </Button>
          ) : undefined
        }
      >
        {info?.canRelaunch ? (
          "TUN mode creates a virtual network adapter, which Windows only allows for administrators."
        ) : (
          <>
            TUN mode needs elevated privileges: <Kbd>{info?.elevationHint}</Kbd>
          </>
        )}
      </Callout>
    );
  }
  if (state.errorCode === "invalid_settings") {
    return (
      <Callout tone="danger" className="mt-5" stacked title="Some settings are invalid">
        {state.error}
      </Callout>
    );
  }
  return (
    <Callout
      tone="danger"
      className="mt-5"
      title="Could not connect"
      action={
        <Button size="sm" variant="secondary" onClick={() => setPage("logs")}>
          View logs
        </Button>
      }
    >
      <span className="selectable break-words">{state.error}</span>
    </Callout>
  );
}

function WarpStatus() {
  const { state, setPage } = useStore();
  const { toast } = useFeedback();
  const [switching, setSwitching] = useState(false);
  const trace = state.trace;
  const connected = state.status === "connected";

  const next = async () => {
    setSwitching(true);
    try {
      const endpoint = await api.NextEndpoint();
      toast("success", "Switched endpoint", `Now connected through ${endpoint}.`);
    } catch (error) {
      toast("danger", "Could not switch endpoint", errorMessage(error));
    } finally {
      setSwitching(false);
    }
  };

  const recheck = async () => {
    try {
      const result = await api.CheckConnection();
      if (!result.ok) toast("warning", "WARP check failed", result.error);
    } catch (error) {
      toast("danger", "WARP check failed", errorMessage(error));
    }
  };

  return (
    <Card className="flex flex-col">
      <CardHeader
        title="WARP status"
        icon={connected && trace.ok ? <ShieldCheck className="size-4" /> : <Globe2 className="size-4" />}
        actions={
          connected && (
            <>
              <Button
                size="sm"
                variant="ghost"
                icon={<SkipForward className="size-3.5" />}
                onClick={next}
                loading={switching}
                title="Switch to the next endpoint"
              >
                Next
              </Button>
              <Button
                size="sm"
                variant="ghost"
                icon={<RefreshCw className={state.checking ? "size-3.5 animate-spin" : "size-3.5"} />}
                onClick={recheck}
                disabled={state.checking || switching}
              >
                Check
              </Button>
            </>
          )
        }
      />
      <div className="flex-1 px-5 pb-5">
        {!connected ? (
          <p className="text-[12.5px] leading-relaxed text-faint">Connect to see your WARP address, location and data center.</p>
        ) : state.checking && !trace.checkedAt ? (
          <div className="flex items-center gap-2.5 text-[13px] text-muted">
            <Spinner /> Verifying the tunnel…
          </div>
        ) : trace.ok ? (
          <div className="space-y-3">
            <div className="flex items-center gap-2">
              <Badge tone={trace.warp === "off" ? "warning" : "success"} dot>
                {trace.warp === "plus" ? "WARP+ active" : trace.warp === "on" ? "WARP active" : `warp=${trace.warp || "unknown"}`}
              </Badge>
            </div>
            <InfoRow icon={<Zap className="size-3.5" />} label="IP address">
              <span className="selectable font-mono text-[12.5px]">{trace.ip}</span>
            </InfoRow>
            <InfoRow icon={<MapPin className="size-3.5" />} label="Location">
              {countryName(trace.location)} <span className="text-faint">({trace.location})</span>
            </InfoRow>
            <InfoRow icon={<Server className="size-3.5" />} label="Data center">
              <span className="font-mono text-[12.5px]">{trace.colo}</span>
            </InfoRow>
            <InfoRow icon={<Gauge className="size-3.5" />} label="Response time">
              <span className="tabular">{trace.latencyMs} ms</span>
            </InfoRow>
            <InfoRow icon={<Globe2 className="size-3.5" />} label="Endpoint">
              <span
                className="selectable font-mono text-[12px] text-muted"
                title={state.automatic ? "Chosen automatically" : "Fixed endpoint"}
              >
                {state.endpoint}
              </span>
            </InfoRow>
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-start gap-2.5">
              <ShieldAlert className="mt-0.5 size-4 shrink-0 text-warning" />
              <div className="text-[12.5px] leading-relaxed">
                <div className="font-medium text-fg">WARP is not responding</div>
                <div className="selectable mt-0.5 break-words text-muted">{trace.error || "The tunnel could not be verified."}</div>
              </div>
            </div>
            <p className="text-[12px] leading-relaxed text-faint">
              {state.automatic
                ? "Harp switches to another endpoint automatically. A scan finds the ones that work on this network."
                : "This endpoint may be blocked on this network. A scan finds the ones that work here."}
            </p>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="primary" icon={<SkipForward className="size-3.5" />} onClick={next} loading={switching}>
                Try next endpoint
              </Button>
              <Button size="sm" variant="secondary" icon={<ArrowRight className="size-3.5" />} onClick={() => setPage("endpoints")}>
                Scan
              </Button>
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}

function InfoRow({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <div className="flex items-center gap-3 text-[13px]">
      <span className="flex size-6 items-center justify-center rounded-md bg-panel-2 text-faint">{icon}</span>
      <span className="w-28 shrink-0 text-muted">{label}</span>
      <span className="min-w-0 truncate text-fg">{children}</span>
    </div>
  );
}

function Metric({
  icon,
  label,
  value,
  hint,
  tone,
  action,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  hint?: string;
  tone?: "accent" | "cyan";
  action?: ReactNode;
}) {
  return (
    <Card className="px-5 py-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2 text-[12px] font-medium text-muted">
          <span className={tone === "accent" ? "text-accent" : tone === "cyan" ? "text-cyan" : "text-faint"}>{icon}</span>
          {label}
        </div>
        {action}
      </div>
      <div className="tabular mt-2 text-[22px] font-semibold tracking-tight">{value}</div>
      <div className="tabular mt-0.5 h-4 font-mono text-[11.5px] text-faint">{hint}</div>
    </Card>
  );
}

function Legend({ color, label, value }: { color: string; label: string; value: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className={`size-2 rounded-full ${color}`} />
      <span className="text-muted">{label}</span>
      <span className="tabular w-20 text-right font-mono text-fg">{value}</span>
    </div>
  );
}

function Welcome({ onSetup }: { onSetup: () => void }) {
  return (
    <div className="flex min-h-[calc(100vh-120px)] items-center justify-center">
      <div className="max-w-lg text-center">
        <div className="mb-6 flex justify-center">
          <Logo size={84} />
        </div>
        <h1 className="text-[28px] font-semibold tracking-tight">
          Welcome to <span className="gradient-text">Harp</span>
        </h1>
        <p className="mt-3 text-[14px] leading-relaxed text-muted">
          A fast, private connection through Cloudflare WARP — as a local proxy or for your whole system, with routing rules for
          every site and app.
        </p>
        <div className="mt-8 flex justify-center gap-3">
          <Button variant="primary" size="lg" icon={<UserPlus className="size-4" />} onClick={onSetup}>
            Set up your WARP account
          </Button>
        </div>
        <p className="mt-4 text-[12px] text-faint">A free account is created in seconds. No sign-up required.</p>
      </div>
    </div>
  );
}
