import { Activity, Pause, Play, Search, X, XCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { Badge, Button, Card, EmptyState, IconButton, Input, PageHeader, Segmented, useFeedback } from "../components/ui";
import { api, errorMessage, type Connection } from "../lib/api";
import { formatBytes, formatShortDuration } from "../lib/format";
import { useNow, useStore } from "../lib/store";

type Filter = "all" | "warp" | "direct";

function outboundTone(outbound: string) {
  if (outbound === "warp") return "accent" as const;
  if (outbound === "direct") return "success" as const;
  return "neutral" as const;
}

export function Connections() {
  const { state } = useStore();
  const { toast, confirm } = useFeedback();
  const [connections, setConnections] = useState<Connection[]>([]);
  const [paused, setPaused] = useState(false);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const now = useNow(1000);
  const connected = state.status === "connected";

  useEffect(() => {
    if (!connected) {
      setConnections([]);
      return;
    }
    if (paused) return;
    let active = true;
    const load = async () => {
      try {
        const list = await api.GetConnections();
        if (active) setConnections(list ?? []);
      } catch {
        /* the next poll retries */
      }
    };
    load();
    const id = window.setInterval(load, 1000);
    return () => {
      active = false;
      window.clearInterval(id);
    };
  }, [connected, paused]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return connections
      .filter((c) => filter === "all" || c.outbound === filter)
      .filter(
        (c) =>
          !q ||
          [c.host, c.destination, c.process, c.rule, c.protocol, c.outbound, c.inbound].some((v) => v?.toLowerCase().includes(q)),
      )
      .sort((a, b) => b.start - a.start);
  }, [connections, query, filter]);

  const totals = useMemo(
    () => connections.reduce((acc, c) => ({ up: acc.up + c.upload, down: acc.down + c.download }), { up: 0, down: 0 }),
    [connections],
  );

  const closeOne = async (id: string) => {
    try {
      await api.CloseConnection(id);
      setConnections((all) => all.filter((c) => c.id !== id));
    } catch (error) {
      toast("danger", "Could not close the connection", errorMessage(error));
    }
  };

  const closeAll = async () => {
    if (!(await confirm({ title: "Close all connections?", description: "Applications will reconnect automatically.", confirmLabel: "Close all", danger: true }))) return;
    await api.CloseAllConnections();
    setConnections([]);
  };

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Connections"
        description="Live view of every connection handled by Harp and the rule that routed it."
        actions={
          <>
            <Button
              variant="secondary"
              icon={paused ? <Play className="size-4" /> : <Pause className="size-4" />}
              onClick={() => setPaused((p) => !p)}
              disabled={!connected}
            >
              {paused ? "Resume" : "Pause"}
            </Button>
            <Button variant="danger" icon={<XCircle className="size-4" />} onClick={closeAll} disabled={!connected || connections.length === 0}>
              Close all
            </Button>
          </>
        }
      />

      <Card className="flex min-h-0 flex-1 flex-col">
        <div className="flex items-center gap-3 border-b border-line px-4 py-3">
          <Input
            className="w-80"
            leading={<Search className="size-4" />}
            placeholder="Filter by host, app, rule…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            trailing={
              query && (
                <IconButton label="Clear" className="size-6" onClick={() => setQuery("")}>
                  <X className="size-3.5" />
                </IconButton>
              )
            }
          />
          <Segmented<Filter>
            size="sm"
            value={filter}
            onChange={setFilter}
            options={[
              { value: "all", label: "All" },
              { value: "warp", label: "WARP" },
              { value: "direct", label: "Direct" },
            ]}
          />
          <div className="ml-auto flex items-center gap-4 text-[12px] text-muted">
            <span>
              <span className="tabular font-medium text-fg">{visible.length}</span> of {connections.length}
            </span>
            <span className="tabular font-mono">↓ {formatBytes(totals.down)}</span>
            <span className="tabular font-mono">↑ {formatBytes(totals.up)}</span>
          </div>
        </div>

        {!connected ? (
          <EmptyState icon={<Activity className="size-5" />} title="Not connected" description="Connections appear here while Harp is running." />
        ) : visible.length === 0 ? (
          <EmptyState
            icon={<Activity className="size-5" />}
            title={connections.length === 0 ? "No active connections" : "Nothing matches the filter"}
            description={connections.length === 0 ? "Open a website or app that uses Harp." : undefined}
          />
        ) : (
          <div className="min-h-0 flex-1 overflow-auto">
            <table className="w-full table-fixed text-[12.5px]">
              <thead className="sticky top-0 z-10 bg-panel text-left text-[11px] font-semibold tracking-wide text-faint uppercase">
                <tr className="border-b border-line">
                  <th className="w-[34%] px-4 py-2.5">Destination</th>
                  <th className="w-[12%] px-2 py-2.5">Route</th>
                  <th className="w-[22%] px-2 py-2.5">Rule</th>
                  <th className="w-[10%] px-2 py-2.5 text-right">Download</th>
                  <th className="w-[10%] px-2 py-2.5 text-right">Upload</th>
                  <th className="w-[8%] px-2 py-2.5 text-right">Time</th>
                  <th className="w-[4%] px-2 py-2.5" />
                </tr>
              </thead>
              <tbody className="divide-y divide-line">
                {visible.slice(0, 500).map((c) => (
                  <tr key={c.id} className="group hover:bg-hover/60">
                    <td className="px-4 py-2">
                      <div className="flex items-center gap-2">
                        <span className="selectable truncate font-medium text-fg" title={c.destination}>
                          {c.host}
                        </span>
                        {c.protocol && (
                          <span className="shrink-0 rounded bg-panel-2 px-1.5 py-px text-[10.5px] text-muted uppercase">{c.protocol}</span>
                        )}
                      </div>
                      <div className="mt-0.5 truncate text-[11.5px] text-faint" title={c.processPath}>
                        {c.network.toUpperCase()} · {c.destination}
                        {c.process && ` · ${c.process}`}
                      </div>
                    </td>
                    <td className="px-2 py-2">
                      <Badge tone={outboundTone(c.outbound)}>{c.outbound === "warp" ? "WARP" : c.outbound === "direct" ? "Direct" : c.outbound}</Badge>
                    </td>
                    <td className="truncate px-2 py-2 font-mono text-[11.5px] text-muted" title={c.rule}>
                      {c.rule}
                    </td>
                    <td className="tabular px-2 py-2 text-right font-mono text-[12px]">{formatBytes(c.download)}</td>
                    <td className="tabular px-2 py-2 text-right font-mono text-[12px]">{formatBytes(c.upload)}</td>
                    <td className="tabular px-2 py-2 text-right text-muted">{formatShortDuration(now - c.start)}</td>
                    <td className="px-2 py-2 text-right">
                      <IconButton label="Close connection" tone="danger" className="size-7 opacity-0 group-hover:opacity-100" onClick={() => closeOne(c.id)}>
                        <X className="size-3.5" />
                      </IconButton>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}
