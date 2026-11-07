import {
  Activity,
  Cable,
  Globe,
  LayoutDashboard,
  ListTree,
  MonitorSmartphone,
  Radar,
  ScrollText,
  Settings as SettingsIcon,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";
import { useStore, type Page } from "../lib/store";
import { Logo } from "./Logo";
import { cn } from "./ui";

interface NavItem {
  page: Page;
  label: string;
  icon: LucideIcon;
}

const SECTIONS: { title?: string; items: NavItem[] }[] = [
  {
    items: [
      { page: "dashboard", label: "Dashboard", icon: LayoutDashboard },
      { page: "connections", label: "Connections", icon: Activity },
      { page: "rules", label: "Routing rules", icon: ListTree },
    ],
  },
  {
    title: "Configure",
    items: [
      { page: "proxy", label: "Proxy", icon: Cable },
      { page: "tun", label: "TUN mode", icon: MonitorSmartphone },
      { page: "dns", label: "DNS", icon: Globe },
      { page: "endpoints", label: "Endpoints", icon: Radar },
      { page: "account", label: "WARP & account", icon: ShieldCheck },
    ],
  },
  {
    title: "System",
    items: [
      { page: "logs", label: "Logs", icon: ScrollText },
      { page: "settings", label: "Settings", icon: SettingsIcon },
    ],
  },
];

export function Sidebar() {
  const { page, setPage, state, info, stats, scan } = useStore();
  const connected = state.status === "connected";
  const degraded = connected && state.trace.checkedAt > 0 && !state.trace.ok;
  const busy = state.status === "connecting" || state.status === "disconnecting";
  const connectionCount = connected ? (stats[stats.length - 1]?.connections ?? 0) : 0;
  const mac = info?.os === "darwin";

  return (
    <aside className="flex w-[232px] shrink-0 flex-col border-r border-line bg-sidebar">
      <div data-drag className={cn("flex items-center gap-2.5 px-5 pb-5", mac ? "pt-12" : "pt-5")}>
        <Logo size={30} />
        <div className="leading-tight">
          <div className="text-[15px] font-semibold tracking-tight">Harp</div>
          <div className="text-[11px] text-faint">WARP client</div>
        </div>
      </div>

      <nav className="flex-1 space-y-5 overflow-y-auto px-3">
        {SECTIONS.map((section, index) => (
          <div key={index}>
            {section.title && (
              <div className="px-2.5 pb-1.5 text-[10.5px] font-semibold tracking-[0.08em] text-faint uppercase">
                {section.title}
              </div>
            )}
            <div className="space-y-0.5">
              {section.items.map((item) => {
                const active = page === item.page;
                const Icon = item.icon;
                return (
                  <button
                    key={item.page}
                    onClick={() => setPage(item.page)}
                    className={cn(
                      "group relative flex h-9 w-full items-center gap-2.5 rounded-[10px] px-2.5 text-[13px] font-medium transition-colors",
                      active ? "bg-panel text-fg shadow-card ring-1 ring-line" : "text-muted hover:bg-hover hover:text-fg",
                    )}
                  >
                    {active && <span className="absolute top-2 bottom-2 left-0 w-[3px] rounded-r-full bg-accent" />}
                    <Icon className={cn("size-[17px]", active ? "text-accent" : "text-faint group-hover:text-muted")} />
                    <span className="flex-1 text-left">{item.label}</span>
                    {item.page === "endpoints" && scan.running && <span className="size-1.5 animate-pulse rounded-full bg-accent" />}
                    {item.page === "connections" && connectionCount > 0 && (
                      <span className="tabular rounded-md bg-panel-2 px-1.5 text-[11px] text-muted">{connectionCount}</span>
                    )}
                  </button>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="m-3 rounded-xl border border-line bg-panel p-3">
        <div className="flex items-center gap-2.5">
          <span className="relative flex size-2.5">
            {connected && !degraded && <span className="absolute inline-flex size-full animate-ping rounded-full bg-success opacity-60" />}
            <span
              className={cn(
                "relative inline-flex size-2.5 rounded-full",
                connected ? (degraded ? "bg-warning" : "bg-success") : busy ? "bg-warning" : state.status === "error" ? "bg-danger" : "bg-faint",
              )}
            />
          </span>
          <div className="min-w-0 flex-1">
            <div className="text-[12.5px] font-medium">
              {connected
                ? degraded
                  ? "No response"
                  : "Protected"
                : busy
                  ? state.status === "connecting"
                    ? "Connecting…"
                    : "Disconnecting…"
                  : state.status === "error"
                    ? "Connection failed"
                    : "Not connected"}
            </div>
            <div className="text-[11px] text-faint">
              {state.mode === "tun" ? "TUN mode" : "Proxy mode"}
              {connected && state.trace.warp === "plus" ? " · WARP+" : ""}
            </div>
          </div>
        </div>
      </div>
      <div className="px-5 pb-4 text-[10.5px] text-faint">
        {info ? (/^\d/.test(info.version) ? `Version ${info.version}` : `Development build`) : "…"}
      </div>
    </aside>
  );
}
