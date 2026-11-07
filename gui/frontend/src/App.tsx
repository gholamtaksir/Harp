import { AlertTriangle } from "lucide-react";
import type { ReactNode } from "react";
import { Logo } from "./components/Logo";
import { SaveBar } from "./components/SaveBar";
import { Sidebar } from "./components/Sidebar";
import { Spinner } from "./components/ui";
import { useStore, type Page } from "./lib/store";
import { Account } from "./pages/Account";
import { Connections } from "./pages/Connections";
import { Dashboard } from "./pages/Dashboard";
import { Dns } from "./pages/Dns";
import { Endpoints } from "./pages/Endpoints";
import { Logs } from "./pages/Logs";
import { Proxy } from "./pages/Proxy";
import { Rules } from "./pages/Rules";
import { Settings } from "./pages/Settings";
import { Tun } from "./pages/Tun";

const PAGES: Record<Page, () => ReactNode> = {
  dashboard: Dashboard,
  connections: Connections,
  rules: Rules,
  proxy: Proxy,
  tun: Tun,
  dns: Dns,
  endpoints: Endpoints,
  account: Account,
  logs: Logs,
  settings: Settings,
};

/** Pages that manage their own scrolling and fill the window height. */
const FULL_HEIGHT: Page[] = ["connections", "logs"];

export function App() {
  const { ready, loadError, page, info } = useStore();

  if (!ready) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4">
        <Logo size={56} />
        {loadError ? (
          <div className="flex items-center gap-2 text-[13px] text-danger">
            <AlertTriangle className="size-4" /> {loadError}
          </div>
        ) : (
          <Spinner />
        )}
      </div>
    );
  }

  const Current = PAGES[page] ?? Dashboard;
  const full = FULL_HEIGHT.includes(page);
  const mac = info?.os === "darwin";

  return (
    <div className="flex h-full">
      <Sidebar />
      <main className="relative flex min-w-0 flex-1 flex-col">
        {mac && <div data-drag className="h-8 shrink-0" />}
        <div className={full ? "flex min-h-0 flex-1 flex-col px-8 pt-7 pb-6" : "min-h-0 flex-1 overflow-y-auto px-8 pt-7 pb-8"}>
          <div key={page} className={full ? "animate-fade-in flex min-h-0 flex-1 flex-col" : "animate-fade-in mx-auto max-w-[1180px]"}>
            <Current />
          </div>
        </div>
        <SaveBar />
      </main>
    </div>
  );
}
