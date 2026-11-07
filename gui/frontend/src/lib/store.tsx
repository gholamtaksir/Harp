import { Events } from "@wailsio/runtime";
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  api,
  errorMessage,
  HarpLogs,
  HarpScan,
  HarpSettings,
  HarpState,
  HarpStats,
  type AppInfo,
  type EndpointResult,
  type LogEntry,
  type ScanState,
  type Settings,
  type State,
  type Stats,
} from "./api";

const MAX_LOGS = 3000;
const MAX_STATS = 300;

export type Page =
  | "dashboard"
  | "connections"
  | "rules"
  | "proxy"
  | "tun"
  | "dns"
  | "endpoints"
  | "account"
  | "logs"
  | "settings";

const EMPTY_STATE: State = {
  status: "disconnected",
  mode: "proxy",
  error: "",
  errorCode: "",
  connectedAt: 0,
  endpoint: "",
  automatic: false,
  listeners: [],
  notice: "",
  trace: { ok: false, ip: "", location: "", colo: "", warp: "", tls: "", latencyMs: 0, error: "", checkedAt: 0 },
  checking: false,
  elevated: false,
  hasAccount: false,
};

/** ScanState with its results list always present. */
export type Scan = ScanState & { results: EndpointResult[] };

const EMPTY_SCAN: Scan = {
  running: false,
  phase: "",
  done: 0,
  total: 0,
  probed: 0,
  candidates: 0,
  found: 0,
  size: "",
  automatic: false,
  startedAt: 0,
  finishedAt: 0,
  error: "",
  last: "",
  results: [],
};

function normalizeScan(scan: ScanState): Scan {
  return { ...scan, results: scan.results ?? [] };
}

/** Fills null arrays so that components can rely on them. */
export function normalizeSettings(settings: Settings): Settings {
  return {
    ...settings,
    proxy: {
      ...settings.proxy,
      listeners: settings.proxy.listeners ?? [],
      users: settings.proxy.users ?? [],
    },
    tun: {
      ...settings.tun,
      includeRoutes: settings.tun.includeRoutes ?? [],
      excludeRoutes: settings.tun.excludeRoutes ?? [],
    },
    routing: { ...settings.routing, rules: settings.routing.rules ?? [] },
  };
}

function same(a: unknown, b: unknown) {
  return JSON.stringify(a) === JSON.stringify(b);
}

interface StoreValue {
  ready: boolean;
  loadError: string;
  info: AppInfo | null;
  state: State;
  stats: Stats[];
  settings: Settings | null;
  draft: Settings | null;
  dirty: boolean;
  errors: Record<string, string>;
  saving: boolean;
  logs: LogEntry[];
  scan: Scan;
  page: Page;
  setPage: (page: Page) => void;
  updateDraft: (update: (draft: Settings) => Settings) => void;
  replaceDraft: (settings: Settings) => void;
  save: () => Promise<{ ok: boolean; error?: string }>;
  discard: () => void;
  clearLogs: () => Promise<void>;
  refreshInfo: () => Promise<void>;
}

const StoreContext = createContext<StoreValue | null>(null);

export function StoreProvider({ children }: { children: ReactNode }) {
  const [ready, setReady] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [info, setInfo] = useState<AppInfo | null>(null);
  const [state, setState] = useState<State>(EMPTY_STATE);
  const [stats, setStats] = useState<Stats[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [draft, setDraft] = useState<Settings | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [scan, setScan] = useState<Scan>(EMPTY_SCAN);
  const [page, setPageState] = useState<Page>(() => {
    try {
      return (localStorage.getItem("harp.page") as Page) || "dashboard";
    } catch {
      return "dashboard";
    }
  });

  const draftRef = useRef<Settings | null>(null);
  const settingsRef = useRef<Settings | null>(null);
  draftRef.current = draft;
  settingsRef.current = settings;

  const setPage = useCallback((next: Page) => {
    setPageState(next);
    try {
      localStorage.setItem("harp.page", next);
    } catch {
      /* storage unavailable */
    }
  }, []);

  const refreshInfo = useCallback(async () => {
    setInfo(await api.GetAppInfo());
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [appInfo, currentState, currentSettings, history, currentLogs, currentScan] = await Promise.all([
          api.GetAppInfo(),
          api.GetState(),
          api.GetSettings(),
          api.GetStatsHistory(),
          api.GetLogs(),
          api.GetScan(),
        ]);
        if (cancelled) return;
        const normalized = normalizeSettings(currentSettings);
        setInfo(appInfo);
        setState(currentState);
        setSettings(normalized);
        setDraft(normalized);
        setStats(history ?? []);
        setLogs(currentLogs ?? []);
        setScan(normalizeScan(currentScan));
        setReady(true);
      } catch (error) {
        setLoadError(errorMessage(error));
      }
    })();

    const offState = Events.On(HarpState, (event) => setState(event.data));
    const offStats = Events.On(HarpStats, (event) => {
      const next = event.data;
      setStats((previous) => {
        const merged = previous.length >= MAX_STATS ? previous.slice(previous.length - MAX_STATS + 1) : previous.slice();
        merged.push(next);
        return merged;
      });
    });
    const offLogs = Events.On(HarpLogs, (event) => {
      const entries = event.data ?? [];
      if (entries.length === 0) return;
      setLogs((previous) => {
        const merged = previous.concat(entries);
        return merged.length > MAX_LOGS ? merged.slice(merged.length - MAX_LOGS) : merged;
      });
    });
    const offScan = Events.On(HarpScan, (event) => setScan(normalizeScan(event.data)));
    const offSettings = Events.On(HarpSettings, (event) => {
      const next = normalizeSettings(event.data);
      const wasDirty = draftRef.current && settingsRef.current && !same(draftRef.current, settingsRef.current);
      setSettings(next);
      if (!wasDirty) {
        setDraft(next);
      } else {
        // Keep unsaved edits but follow mode switches made from the tray.
        setDraft((current) => (current ? { ...current, mode: next.mode } : next));
      }
    });
    return () => {
      cancelled = true;
      offState();
      offStats();
      offLogs();
      offScan();
      offSettings();
    };
  }, []);

  // Clear the traffic history when a new session starts.
  const connectedAt = state.connectedAt;
  const previousConnectedAt = useRef(connectedAt);
  useEffect(() => {
    if (connectedAt !== previousConnectedAt.current && connectedAt !== 0) setStats([]);
    previousConnectedAt.current = connectedAt;
  }, [connectedAt]);

  const updateDraft = useCallback((update: (draft: Settings) => Settings) => {
    setDraft((current) => (current ? update(current) : current));
  }, []);

  const replaceDraft = useCallback((next: Settings) => setDraft(normalizeSettings(next)), []);

  const dirty = useMemo(() => !!draft && !!settings && !same(draft, settings), [draft, settings]);

  const save = useCallback(async () => {
    const current = draftRef.current;
    if (!current) return { ok: false };
    setSaving(true);
    try {
      const result = await api.SaveSettings(current);
      if (!result.ok) {
        const fieldErrors: Record<string, string> = {};
        for (const field of result.fields ?? []) fieldErrors[field.field] = field.message;
        setErrors(fieldErrors);
        return { ok: false, error: (result.fields ?? []).map((f) => f.message).join("; ") };
      }
      setErrors({});
      setSettings(current);
      return { ok: true };
    } catch (error) {
      return { ok: false, error: errorMessage(error) };
    } finally {
      setSaving(false);
    }
  }, []);

  const discard = useCallback(() => {
    setDraft(settingsRef.current);
    setErrors({});
  }, []);

  const clearLogs = useCallback(async () => {
    await api.ClearLogs();
    setLogs([]);
  }, []);

  // Apply the theme, previewing unsaved changes.
  const theme = draft?.app.theme ?? "system";
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => {
      const dark = theme === "dark" || (theme !== "light" && media.matches);
      document.documentElement.classList.toggle("dark", dark);
    };
    apply();
    media.addEventListener("change", apply);
    return () => media.removeEventListener("change", apply);
  }, [theme]);

  const value: StoreValue = {
    ready,
    loadError,
    info,
    state,
    stats,
    settings,
    draft,
    dirty,
    errors,
    saving,
    logs,
    scan,
    page,
    setPage,
    updateDraft,
    replaceDraft,
    save,
    discard,
    clearLogs,
    refreshInfo,
  };
  return <StoreContext.Provider value={value}>{children}</StoreContext.Provider>;
}

export function useStore(): StoreValue {
  const value = useContext(StoreContext);
  if (!value) throw new Error("useStore outside StoreProvider");
  return value;
}

/** The editable settings draft; only use below a ready store. */
export function useDraft() {
  const store = useStore();
  return { draft: store.draft as Settings, update: store.updateDraft, errors: store.errors };
}

/** Re-renders every interval milliseconds and returns the current time. */
export function useNow(interval = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), interval);
    return () => window.clearInterval(id);
  }, [interval]);
  return now;
}
