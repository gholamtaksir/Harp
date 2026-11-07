import {
  Activity,
  CircleCheck,
  FlaskConical,
  Gauge,
  Pin,
  Play,
  Radar,
  Route,
  Shuffle,
  Square,
  Trash2,
  Waves,
} from "lucide-react";
import { useMemo, useState, type ReactNode } from "react";
import {
  Badge,
  Button,
  Callout,
  Card,
  CardHeader,
  Checkbox,
  CopyButton,
  EmptyState,
  Field,
  Input,
  PageHeader,
  Segmented,
  SettingList,
  SettingRow,
  Spinner,
  cn,
  useFeedback,
} from "../components/ui";
import { api, errorMessage, type EndpointResult } from "../lib/api";
import { countryName } from "../lib/format";
import { useDraft, useNow, useStore, type Scan } from "../lib/store";

export const ENDPOINT_IPS = [
  "162.159.192.1",
  "162.159.195.1",
  "188.114.96.1",
  "188.114.97.1",
  "188.114.98.1",
  "188.114.99.1",
];

export const ENDPOINT_PORTS = [
  2408, 500, 1701, 4500, 854, 859, 864, 878, 880, 890, 894, 903, 908, 928, 934, 939, 942, 943, 945, 946, 955, 968, 987,
  988, 1002, 1010, 1014, 1018, 1070, 1074, 1180, 1387, 1843, 2371, 2506, 3138, 3476, 3581, 3854, 4177, 4198, 4233, 5279,
  5956, 7103, 7152, 7156, 7281, 7559, 8319, 8742, 8854, 8886,
];

const SIZES = [
  { value: "quick", label: "Quick", count: 300, hint: "About 10 seconds" },
  { value: "standard", label: "Standard", count: 1000, hint: "About 20 seconds" },
  { value: "thorough", label: "Thorough", count: 3000, hint: "About a minute" },
] as const;

type Size = (typeof SIZES)[number]["value"];

const VISIBLE_ROWS = 40;

export function Endpoints() {
  const { scan } = useStore();
  return (
    <div className="space-y-5">
      <PageHeader
        title="Endpoints"
        description="WARP servers listen on hundreds of addresses and ports. Some networks block a few of them, or let the handshake through but drop the traffic. Find the ones that work here."
      />
      <Selection />
      <Scanner />
      <Results scan={scan} />
    </div>
  );
}

// ---------------------------------------------------------------- Selection

function Selection() {
  const { state, scan } = useStore();
  const { draft, update, errors } = useDraft();
  const endpoint = draft.warp.endpoint;
  const fixed = endpoint !== "";
  const [host, port] = splitEndpoint(endpoint);
  const setEndpoint = (value: string) => update((d) => ({ ...d, warp: { ...d.warp, endpoint: value } }));
  const connected = state.status === "connected";

  return (
    <Card>
      <CardHeader
        icon={<Route className="size-4" />}
        title="Endpoint selection"
        description="Which WARP server Harp connects to."
        actions={
          connected && state.endpoint ? (
            <Badge tone="success" dot>
              Using <span className="selectable font-mono">{state.endpoint}</span>
              {state.automatic ? " · automatic" : ""}
            </Badge>
          ) : undefined
        }
      />
      <SettingList>
        <SettingRow
          title="Mode"
          description={
            fixed
              ? "Always connect to the address below, even if it stops working."
              : "Harp uses the endpoint that last carried traffic, otherwise the fastest one your scans found, and switches to another one when traffic stops getting through."
          }
          error={errors["warp.endpoint"]}
        >
          <Segmented
            value={fixed ? "fixed" : "auto"}
            onChange={(value) => setEndpoint(value === "auto" ? "" : suggestedEndpoint())}
            options={[
              { value: "auto", label: "Automatic", icon: <Shuffle className="size-3.5" /> },
              { value: "fixed", label: "Fixed", icon: <Pin className="size-3.5" /> },
            ]}
          />
        </SettingRow>
        {fixed && (
          <div className="flex items-end gap-3 px-5 py-3.5">
            <Field label="Server IP or host" className="flex-1">
              <Input
                mono
                list="harp-endpoint-ips"
                value={host}
                onChange={(event) => setEndpoint(`${event.target.value.trim()}:${port}`)}
              />
              <datalist id="harp-endpoint-ips">
                {ENDPOINT_IPS.map((ip) => (
                  <option key={ip} value={ip} />
                ))}
              </datalist>
            </Field>
            <Field label="Port" className="w-36">
              <Input
                mono
                list="harp-endpoint-ports"
                value={port}
                onChange={(event) => setEndpoint(`${host}:${event.target.value.trim()}`)}
              />
              <datalist id="harp-endpoint-ports">
                {ENDPOINT_PORTS.map((p) => (
                  <option key={p} value={p} />
                ))}
              </datalist>
            </Field>
          </div>
        )}
      </SettingList>
    </Card>
  );

  /** The endpoint in use, else the best scan result, else a common one. */
  function suggestedEndpoint() {
    if (state.status === "connected" && state.endpoint) return state.endpoint;
    const working = scan.results.find((r) => r.status === "working") ?? scan.results.find((r) => r.status === "answered");
    return working?.endpoint ?? "188.114.96.1:1701";
  }
}

function splitEndpoint(endpoint: string): [string, string] {
  const index = endpoint.lastIndexOf(":");
  return index > 0 ? [endpoint.slice(0, index), endpoint.slice(index + 1)] : [endpoint, ""];
}

// ---------------------------------------------------------------- Scanner

const PHASES: Record<string, string> = {
  probing: "Sending handshakes",
  measuring: "Measuring the fastest",
  verifying: "Testing traffic",
};

function Scanner() {
  const { scan, state, setPage } = useStore();
  const { toast, confirm } = useFeedback();
  const [size, setSize] = useState<Size>("standard");
  const [ipv6, setIpv6] = useState(false);
  const [starting, setStarting] = useState(false);
  const connected = state.status === "connected" || state.status === "connecting";

  const start = async () => {
    if (!state.hasAccount) {
      setPage("account");
      return;
    }
    if (connected) {
      const ok = await confirm({
        title: "Disconnect to scan?",
        description:
          "Harp disconnects while it scans, so that the test uses your real network instead of the tunnel. Connect again afterwards.",
        confirmLabel: "Disconnect and scan",
      });
      if (!ok) return;
    }
    setStarting(true);
    try {
      await api.StartScan({ size, ipv6, verify: 5 });
    } catch (error) {
      toast("danger", "Could not start the scan", errorMessage(error));
    } finally {
      setStarting(false);
    }
  };

  const running = scan.running;
  const percent = scan.total > 0 ? Math.min(100, Math.round((scan.done / scan.total) * 100)) : 0;
  const selected = SIZES.find((s) => s.value === size)!;

  return (
    <Card>
      <CardHeader
        icon={<Radar className="size-4" />}
        title="Scan"
        description="Sends a WireGuard handshake from your account to random WARP addresses and ports, measures the ones that answer, then sends real traffic through the fastest."
      />
      <div className="border-t border-line px-5 py-4">
        {running ? (
          <div className="space-y-3">
            <div className="flex items-center gap-3">
              <Spinner className="text-accent" />
              <div className="min-w-0 flex-1">
                <div className="text-[13px] font-medium text-fg">
                  {PHASES[scan.phase] ?? "Starting"}
                  {scan.automatic && <span className="font-normal text-muted"> · started while connecting</span>}
                </div>
                <div className="tabular mt-0.5 text-[12px] text-muted">{progressText(scan)}</div>
              </div>
              <Button variant="secondary" size="sm" icon={<Square className="size-3" />} onClick={() => api.StopScan()}>
                Stop
              </Button>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-panel-2">
              <div
                className="h-full rounded-full transition-[width] duration-300"
                style={{ width: `${percent}%`, background: "linear-gradient(90deg, var(--accent), var(--cyan))" }}
              />
            </div>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-4">
            <Segmented
              value={size}
              onChange={setSize}
              options={SIZES.map((s) => ({ value: s.value, label: `${s.label} · ${s.count.toLocaleString("en-US")}` }))}
            />
            <Checkbox checked={ipv6} onChange={setIpv6}>
              Include IPv6
            </Checkbox>
            <span className="text-[12px] text-faint">{selected.hint}</span>
            <div className="flex-1" />
            <Button variant="primary" icon={<Play className="size-3.5" />} loading={starting} onClick={start}>
              Start scan
            </Button>
          </div>
        )}
      </div>
      {!running && connected && (
        <div className="px-5 pb-4">
          <Callout tone="accent">Scanning disconnects Harp first, so that it tests your network rather than the tunnel.</Callout>
        </div>
      )}
    </Card>
  );
}

function progressText(scan: Scan) {
  const n = (value: number) => value.toLocaleString("en-US");
  switch (scan.phase) {
    case "probing":
      return `${n(scan.done)} of ${n(scan.total)} tried · ${n(scan.found)} answered`;
    case "measuring":
      return `${n(scan.done)} of ${n(scan.total)} measured · ${n(scan.found)} answered in total`;
    case "verifying":
      return `${n(scan.done)} of ${n(scan.total)} tested · ${scan.results.filter((r) => r.status === "working").length} working`;
  }
  return "Preparing…";
}

// ---------------------------------------------------------------- Results

const STATUS: Record<string, { label: string; tone: "success" | "neutral" | "danger" | "warning"; hint: string }> = {
  working: { label: "Working", tone: "success", hint: "Traffic got through WARP" },
  answered: { label: "Answered", tone: "neutral", hint: "Answered the handshake; traffic not tested yet" },
  blocked: { label: "Blocked", tone: "danger", hint: "Answered the handshake, but traffic did not get through" },
  unreachable: { label: "No answer", tone: "warning", hint: "Did not answer" },
};

function Results({ scan }: { scan: Scan }) {
  const { state, draft, updateDraft } = useStore();
  const { toast } = useFeedback();
  const now = useNow(30_000);
  const [filter, setFilter] = useState<"all" | "working">("all");
  const [showAll, setShowAll] = useState(false);
  const [testing, setTesting] = useState<Record<string, boolean>>({});
  const [using, setUsing] = useState("");

  const working = scan.results.filter((r) => r.status === "working");
  const rows = useMemo(
    () => (filter === "working" ? scan.results.filter((r) => r.status === "working") : scan.results),
    [scan.results, filter],
  );
  const visible = showAll ? rows : rows.slice(0, VISIBLE_ROWS);
  const fixedEndpoint = draft?.warp.endpoint ?? "";
  const colos = [...new Set(working.map((r) => r.colo).filter(Boolean))];
  const best = scan.results
    .filter((r) => r.ping > 0 && r.status !== "blocked" && r.status !== "unreachable")
    .reduce<EndpointResult | undefined>((a, b) => (!a || b.ping < a.ping ? b : a), undefined);

  const test = async (endpoint: string) => {
    setTesting((current) => ({ ...current, [endpoint]: true }));
    try {
      const result = await api.TestEndpoint(endpoint);
      if (result.status === "working") {
        toast("success", `${endpoint} works`, `Data center ${result.colo}${result.city ? `, ${result.city}` : ""} · ${result.latency} ms`);
      } else {
        toast("warning", `${endpoint} does not carry traffic`, STATUS[result.status]?.hint ?? result.error);
      }
    } catch (error) {
      toast("danger", "Test failed", errorMessage(error));
    } finally {
      setTesting((current) => ({ ...current, [endpoint]: false }));
    }
  };

  const use = async (endpoint: string) => {
    setUsing(endpoint);
    updateDraft((d) => ({ ...d, warp: { ...d.warp, endpoint } }));
    try {
      const result = await api.UseEndpoint(endpoint);
      if (!result.ok) {
        toast("danger", "Could not use this endpoint", (result.fields ?? []).map((f) => f.message).join("; "));
      } else {
        toast("success", `Using ${endpoint}`, state.status === "connected" ? "Reconnecting through it." : "Harp connects through it from now on.");
      }
    } catch (error) {
      toast("danger", "Could not use this endpoint", errorMessage(error));
    } finally {
      setUsing("");
    }
  };

  const clear = async () => {
    try {
      await api.ClearEndpoints();
    } catch (error) {
      toast("danger", "Could not clear the results", errorMessage(error));
    }
  };

  return (
    <Card className="overflow-hidden">
      <CardHeader
        icon={<Activity className="size-4" />}
        title="Results"
        description={summary(scan, working.length, now)}
        actions={
          scan.results.length > 0 && (
            <>
              <Segmented
                size="sm"
                value={filter}
                onChange={setFilter}
                options={[
                  { value: "all", label: `All ${scan.results.length}` },
                  { value: "working", label: `Working ${working.length}` },
                ]}
              />
              <Button size="sm" variant="ghost" icon={<Trash2 className="size-3.5" />} onClick={clear} disabled={scan.running}>
                Clear
              </Button>
            </>
          )
        }
      />
      {scan.results.length > 0 && (
        <div className="grid grid-cols-4 gap-3 border-t border-line px-5 py-3.5">
          <Stat icon={<CircleCheck className="size-3.5" />} label="Carry traffic" value={String(working.length)} tone="success" />
          <Stat
            icon={<Gauge className="size-3.5" />}
            label="Best ping"
            value={best ? `${Math.round(best.ping)} ms` : "—"}
            tone="accent"
          />
          <Stat
            icon={<Waves className="size-3.5" />}
            label="Answered"
            value={scan.probed > 0 ? `${scan.found.toLocaleString("en-US")} of ${scan.probed.toLocaleString("en-US")}` : String(scan.results.length)}
          />
          <Stat icon={<Radar className="size-3.5" />} label="Data centers" value={colos.length ? colos.join(", ") : "—"} />
        </div>
      )}
      {rows.length === 0 ? (
        <div className="border-t border-line">
          {scan.results.length === 0 ? (
            <EmptyState
              icon={<Radar className="size-5" />}
              title={scan.running ? "Scanning…" : "No endpoints yet"}
              description={
                scan.running
                  ? "Endpoints appear here as they answer."
                  : "Run a scan to find the WARP endpoints that answer on your network and see which ones carry traffic."
              }
            />
          ) : (
            <EmptyState
              icon={<CircleCheck className="size-5" />}
              title="No working endpoint yet"
              description="Test some of the endpoints that answered, or run a scan; it tests the fastest ones automatically."
            />
          )}
        </div>
      ) : (
        <div className="border-t border-line">
          <table className="w-full table-fixed text-[12.5px]">
            <thead className="bg-panel text-left text-[11px] font-semibold tracking-wide text-faint uppercase">
              <tr className="border-b border-line">
                <th className="w-[27%] px-5 py-2.5">Endpoint</th>
                <th className="w-[9%] px-2 py-2.5 text-right">Ping</th>
                <th className="w-[8%] px-2 py-2.5 text-right">Jitter</th>
                <th className="w-[7%] px-2 py-2.5 text-right">Loss</th>
                <th className="w-[20%] px-2 py-2.5 pl-5">Data center</th>
                <th className="w-[12%] px-2 py-2.5">Status</th>
                <th className="w-[17%] px-5 py-2.5" />
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {visible.map((r) => (
                <Row
                  key={r.endpoint}
                  result={r}
                  last={r.endpoint === scan.last}
                  inUse={r.endpoint === fixedEndpoint || (state.status === "connected" && r.endpoint === state.endpoint)}
                  testing={!!testing[r.endpoint]}
                  using={using === r.endpoint}
                  busy={scan.running}
                  onTest={() => test(r.endpoint)}
                  onUse={() => use(r.endpoint)}
                />
              ))}
            </tbody>
          </table>
          {rows.length > VISIBLE_ROWS && (
            <div className="border-t border-line px-5 py-2.5 text-center">
              <button className="text-[12px] font-medium text-accent hover:underline" onClick={() => setShowAll(!showAll)}>
                {showAll ? "Show fewer" : `Show all ${rows.length}`}
              </button>
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

function summary(scan: Scan, working: number, now: number): ReactNode {
  if (scan.running) return "Results update as endpoints answer.";
  if (!scan.finishedAt) {
    return scan.results.length ? "Endpoints Harp has used or tested." : "Nothing scanned yet.";
  }
  const when = timeAgo(now - scan.finishedAt);
  const state = scan.phase === "cancelled" ? "Stopped" : "Last scan";
  return `${state} ${when} · ${scan.found.toLocaleString("en-US")} of ${scan.probed.toLocaleString("en-US")} answered · ${working} carry traffic`;
}

function timeAgo(ms: number): string {
  const minutes = Math.floor(ms / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} h ago`;
  return `${Math.floor(hours / 24)} d ago`;
}

function Stat({ icon, label, value, tone }: { icon: ReactNode; label: string; value: string; tone?: "success" | "accent" }) {
  return (
    <div className="min-w-0 rounded-xl border border-line bg-panel-2 px-3.5 py-2.5">
      <div className="flex items-center gap-1.5 text-[11.5px] font-medium text-muted">
        <span className={tone === "success" ? "text-success" : tone === "accent" ? "text-accent" : "text-faint"}>{icon}</span>
        {label}
      </div>
      <div className="tabular mt-1 truncate text-[15px] font-semibold tracking-tight text-fg" title={value}>
        {value}
      </div>
    </div>
  );
}

function pingTone(ping: number) {
  if (ping <= 0) return "bg-faint";
  if (ping < 150) return "bg-success";
  if (ping < 300) return "bg-warning";
  return "bg-danger";
}

function Row({
  result: r,
  last,
  inUse,
  testing,
  using,
  busy,
  onTest,
  onUse,
}: {
  result: EndpointResult;
  last: boolean;
  inUse: boolean;
  testing: boolean;
  using: boolean;
  busy: boolean;
  onTest: () => void;
  onUse: () => void;
}) {
  const status = STATUS[r.status] ?? STATUS.answered;
  // Endpoints recorded from a connection have no handshake measurements.
  const measured = r.ping > 0;
  return (
    <tr className="group hover:bg-hover/60">
      <td className="px-5 py-2">
        <div className="flex items-center gap-1.5">
          <span className="selectable truncate font-mono text-[12px] text-fg" title={r.endpoint}>
            {r.endpoint}
          </span>
          <span className="opacity-0 transition-opacity group-hover:opacity-100">
            <CopyButton text={r.endpoint} label="Copy endpoint" />
          </span>
        </div>
        {(last || inUse) && (
          <div className="mt-0.5 flex gap-1.5">
            {inUse && <Badge tone="accent">In use</Badge>}
            {last && !inUse && <Badge tone="neutral">Last used</Badge>}
          </div>
        )}
      </td>
      <td className="tabular px-2 py-2 text-right font-mono">
        {measured ? (
          <span className="inline-flex items-center gap-1.5">
            <span className={cn("size-1.5 rounded-full", pingTone(r.ping))} />
            {Math.round(r.ping)} ms
          </span>
        ) : (
          <span className="text-faint">—</span>
        )}
      </td>
      <td className="tabular px-2 py-2 text-right font-mono text-muted">{measured ? `${Math.round(r.jitter)} ms` : "—"}</td>
      <td className={cn("tabular px-2 py-2 text-right font-mono", r.loss > 0 ? "text-warning" : "text-muted")}>
        {measured ? `${Math.round(r.loss)}%` : "—"}
      </td>
      <td className="px-2 py-2 pl-5">
        {r.colo ? (
          <div className="min-w-0">
            <div className="truncate text-fg">
              <span className="font-mono text-[12px] font-semibold">{r.colo}</span>
              {r.city && <span className="text-muted"> · {r.city}</span>}
            </div>
            {r.country && <div className="truncate text-[11.5px] text-faint">{countryName(r.country)}</div>}
          </div>
        ) : (
          <span className="text-faint">{r.status === "answered" ? "Not tested" : "—"}</span>
        )}
      </td>
      <td className="px-2 py-2">
        <span title={r.error || status.hint}>
          <Badge tone={status.tone} dot>
            {status.label}
          </Badge>
        </span>
      </td>
      <td className="px-5 py-2">
        <div className="flex justify-end gap-1.5">
          <Button size="sm" variant="ghost" icon={<FlaskConical className="size-3.5" />} loading={testing} disabled={busy} onClick={onTest}>
            Test
          </Button>
          <Button
            size="sm"
            variant={r.status === "working" ? "subtle" : "secondary"}
            loading={using}
            disabled={busy || inUse || r.status === "blocked" || r.status === "unreachable"}
            onClick={onUse}
          >
            Use
          </Button>
        </div>
      </td>
    </tr>
  );
}
