import { ArrowDownToLine, Download, Pause, Play, ScrollText, Search, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Button, Card, EmptyState, IconButton, Input, PageHeader, Segmented, cn, useFeedback } from "../components/ui";
import { api, errorMessage, isCancelled, type LogEntry } from "../lib/api";
import { formatTime } from "../lib/format";
import { useStore } from "../lib/store";

type Level = "all" | "debug" | "info" | "warn" | "error";

const LEVEL_RANK: Record<string, number> = { trace: 0, debug: 1, info: 2, warn: 3, warning: 3, error: 4, fatal: 5, panic: 5 };
const MIN_RANK: Record<Level, number> = { all: 0, debug: 1, info: 2, warn: 3, error: 4 };

const LEVEL_STYLE: Record<string, string> = {
  trace: "text-faint",
  debug: "text-faint",
  info: "text-cyan",
  warn: "text-warning",
  warning: "text-warning",
  error: "text-danger",
  fatal: "text-danger",
  panic: "text-danger",
};

const VISIBLE_LIMIT = 1500;

export function Logs() {
  const { logs, clearLogs, draft } = useStore();
  const { toast } = useFeedback();
  const [level, setLevel] = useState<Level>("all");
  const [query, setQuery] = useState("");
  const [follow, setFollow] = useState(true);
  const [frozen, setFrozen] = useState<LogEntry[] | null>(null);
  const scroller = useRef<HTMLDivElement>(null);

  const source = frozen ?? logs;
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = source.filter((entry) => (LEVEL_RANK[entry.level] ?? 2) >= MIN_RANK[level] && (!q || entry.message.toLowerCase().includes(q)));
    return filtered.length > VISIBLE_LIMIT ? filtered.slice(filtered.length - VISIBLE_LIMIT) : filtered;
  }, [source, level, query]);

  useEffect(() => {
    if (follow && scroller.current) scroller.current.scrollTop = scroller.current.scrollHeight;
  }, [visible, follow]);

  const exportLogs = async () => {
    try {
      const path = await api.ExportLogs();
      if (path) toast("success", "Logs exported", path);
    } catch (error) {
      if (!isCancelled(error)) toast("danger", "Export failed", errorMessage(error));
    }
  };

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title="Logs"
        description={
          <>
            Messages from Harp and the network engine. The detail level is set in Settings (currently <b className="font-medium text-fg">{draft?.app.logLevel}</b>).
          </>
        }
        actions={
          <>
            <Button variant="secondary" icon={frozen ? <Play className="size-4" /> : <Pause className="size-4" />} onClick={() => setFrozen(frozen ? null : logs.slice())}>
              {frozen ? "Resume" : "Pause"}
            </Button>
            <Button variant="secondary" icon={<Download className="size-4" />} onClick={exportLogs} disabled={logs.length === 0}>
              Export
            </Button>
            <Button variant="danger" icon={<Trash2 className="size-4" />} onClick={() => { setFrozen(null); clearLogs(); }} disabled={logs.length === 0}>
              Clear
            </Button>
          </>
        }
      />
      <Card className="flex min-h-0 flex-1 flex-col">
        <div className="flex items-center gap-3 border-b border-line px-4 py-3">
          <Input
            className="w-80"
            leading={<Search className="size-4" />}
            placeholder="Search messages"
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
          <Segmented<Level>
            size="sm"
            value={level}
            onChange={setLevel}
            options={[
              { value: "all", label: "All" },
              { value: "debug", label: "Debug" },
              { value: "info", label: "Info" },
              { value: "warn", label: "Warnings" },
              { value: "error", label: "Errors" },
            ]}
          />
          <div className="ml-auto flex items-center gap-3 text-[12px] text-muted">
            <span className="tabular">{visible.length} lines</span>
            <Button size="sm" variant={follow ? "subtle" : "ghost"} icon={<ArrowDownToLine className="size-3.5" />} onClick={() => setFollow((f) => !f)}>
              Auto-scroll
            </Button>
          </div>
        </div>
        {visible.length === 0 ? (
          <EmptyState icon={<ScrollText className="size-5" />} title={logs.length === 0 ? "No log messages yet" : "Nothing matches the filter"} />
        ) : (
          <div
            ref={scroller}
            onWheel={(event) => event.deltaY < 0 && setFollow(false)}
            className="selectable min-h-0 flex-1 overflow-auto px-2 py-2 font-mono text-[12px] leading-[1.65]"
          >
            {visible.map((entry) => (
              <div key={entry.id} className="flex gap-3 rounded px-2 hover:bg-hover/60">
                <span className="tabular shrink-0 text-faint">{formatTime(entry.time, true)}</span>
                <span className={cn("w-12 shrink-0 font-semibold uppercase", LEVEL_STYLE[entry.level] ?? "text-muted")}>{entry.level.slice(0, 5)}</span>
                <span className="min-w-0 break-all whitespace-pre-wrap text-fg">{entry.message}</span>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
