import { Cable, Eye, EyeOff, KeyRound, Link2, Monitor, Plus, Trash2, Wand2 } from "lucide-react";
import { useState } from "react";
import {
  Button,
  Callout,
  Card,
  CardHeader,
  CopyButton,
  EmptyState,
  IconButton,
  Input,
  PageHeader,
  Select,
  SettingList,
  SettingRow,
  Switch,
  cn,
} from "../components/ui";
import type { Listener, User } from "../lib/api";
import { randomPassword } from "../lib/format";
import { useDraft, useStore } from "../lib/store";

const LISTENER_TYPES = [
  { value: "mixed", label: "Mixed (SOCKS + HTTP)" },
  { value: "socks", label: "SOCKS5" },
  { value: "http", label: "HTTP" },
];

export function Proxy() {
  const { draft, update, errors } = useDraft();
  const { state } = useStore();
  const proxy = draft.proxy;
  const listeners = proxy.listeners ?? [];
  const users = proxy.users ?? [];

  const setProxy = (patch: Partial<typeof proxy>) => update((d) => ({ ...d, proxy: { ...d.proxy, ...patch } }));
  const setListener = (index: number, patch: Partial<Listener>) =>
    setProxy({ listeners: listeners.map((l, i) => (i === index ? { ...l, ...patch } : l)) });
  const setUser = (index: number, patch: Partial<User>) => setProxy({ users: users.map((u, i) => (i === index ? { ...u, ...patch } : u)) });

  const nextPort = () => {
    const used = new Set(listeners.map((l) => l.port));
    let port = 1080;
    while (used.has(port)) port++;
    return port;
  };

  const exposed = listeners.some((l) => l.enabled && l.address !== "127.0.0.1" && l.address !== "::1" && l.address !== "localhost");
  const credentials = proxy.authEnabled && users[0] ? `${encodeURIComponent(users[0].username)}:${encodeURIComponent(users[0].password)}@` : "";

  return (
    <div className="space-y-5 pb-20">
      <PageHeader
        title="Proxy"
        description="Local proxy servers that applications can use to reach the internet through Harp. They are active in proxy mode, and optionally in TUN mode."
      />

      {state.mode === "tun" && !draft.tun.serveProxy && (
        <Callout tone="accent" title="Proxy servers are off in TUN mode">
          Enable “Also run proxy servers” on the TUN page to use them alongside the tunnel, for example to share WARP with other devices.
        </Callout>
      )}

      <Card>
        <CardHeader
          icon={<Cable className="size-4" />}
          title="Listeners"
          description="Each listener accepts proxy connections on an address and port."
          actions={
            <Button
              size="sm"
              variant="secondary"
              icon={<Plus className="size-3.5" />}
              onClick={() => setProxy({ listeners: [...listeners, { type: "mixed", address: "127.0.0.1", port: nextPort(), enabled: true }] })}
            >
              Add listener
            </Button>
          }
        />
        {listeners.length === 0 ? (
          <EmptyState icon={<Cable className="size-5" />} title="No listeners" description="Add a listener to use proxy mode." className="border-t border-line py-10" />
        ) : (
          <div className="border-t border-line">
            <div className="grid grid-cols-[1.3fr_1.5fr_0.7fr_auto_auto] items-center gap-3 px-5 py-2 text-[11px] font-semibold tracking-wide text-faint uppercase">
              <span>Protocol</span>
              <span>Listen on</span>
              <span>Port</span>
              <span>On</span>
              <span className="w-8" />
            </div>
            {listeners.map((listener, index) => {
              const preset = ["127.0.0.1", "0.0.0.0", "::"].includes(listener.address) ? listener.address : "custom";
              const error = errors[`proxy.listeners[${index}]`];
              return (
                <div key={index} className="border-t border-line px-5 py-2.5">
                  <div className={cn("grid grid-cols-[1.3fr_1.5fr_0.7fr_auto_auto] items-center gap-3", !listener.enabled && "opacity-60")}>
                    <Select value={listener.type} onChange={(event) => setListener(index, { type: event.target.value })}>
                      {LISTENER_TYPES.map((t) => (
                        <option key={t.value} value={t.value}>
                          {t.label}
                        </option>
                      ))}
                    </Select>
                    <div className="flex gap-2">
                      <Select
                        className="flex-1"
                        value={preset}
                        onChange={(event) => setListener(index, { address: event.target.value === "custom" ? "" : event.target.value })}
                      >
                        <option value="127.0.0.1">This device only</option>
                        <option value="0.0.0.0">All networks (IPv4)</option>
                        <option value="::">All networks (IPv4 + IPv6)</option>
                        <option value="custom">Specific address…</option>
                      </Select>
                      {preset === "custom" && (
                        <Input mono className="w-36" value={listener.address} placeholder="192.168.1.10" onChange={(event) => setListener(index, { address: event.target.value.trim() })} />
                      )}
                    </div>
                    <Input
                      mono
                      type="number"
                      min={1}
                      max={65535}
                      value={listener.port || ""}
                      invalid={!!error}
                      onChange={(event) => setListener(index, { port: Number(event.target.value) || 0 })}
                    />
                    <Switch checked={listener.enabled} onChange={(enabled) => setListener(index, { enabled })} label="Enabled" />
                    <IconButton label="Remove listener" tone="danger" onClick={() => setProxy({ listeners: listeners.filter((_, i) => i !== index) })}>
                      <Trash2 className="size-4" />
                    </IconButton>
                  </div>
                  {error && <div className="mt-1.5 text-[12px] text-danger">{error}</div>}
                </div>
              );
            })}
          </div>
        )}
      </Card>

      {exposed && !proxy.authEnabled && (
        <Callout tone="warning" title="Your proxy is reachable from other devices without a password">
          Anyone on your network can use your WARP connection. Turn on authentication below.
        </Callout>
      )}

      <Card>
        <CardHeader icon={<KeyRound className="size-4" />} title="Authentication" description="Require a username and password from clients." actions={<Switch checked={proxy.authEnabled} onChange={(authEnabled) => setProxy({ authEnabled, users: authEnabled && users.length === 0 ? [{ username: "harp", password: randomPassword() }] : users })} />} />
        {proxy.authEnabled && (
          <div className="border-t border-line px-5 py-4">
            <div className="space-y-2.5">
              {users.map((user, index) => (
                <UserRow
                  key={index}
                  user={user}
                  error={errors[`proxy.users[${index}]`]}
                  onChange={(patch) => setUser(index, patch)}
                  onRemove={() => setProxy({ users: users.filter((_, i) => i !== index) })}
                />
              ))}
            </div>
            {errors["proxy.users"] && <div className="mt-2 text-[12px] text-danger">{errors["proxy.users"]}</div>}
            <Button
              className="mt-3"
              size="sm"
              variant="secondary"
              icon={<Plus className="size-3.5" />}
              onClick={() => setProxy({ users: [...users, { username: `user${users.length + 1}`, password: randomPassword() }] })}
            >
              Add user
            </Button>
          </div>
        )}
      </Card>

      <Card>
        <CardHeader icon={<Monitor className="size-4" />} title="System integration" />
        <SettingList>
          <SettingRow
            title="Set as system proxy"
            description="While connected in proxy mode, point this computer's proxy settings at the first Mixed or HTTP listener. Restored on disconnect."
          >
            <Switch checked={proxy.systemProxy} onChange={(systemProxy) => setProxy({ systemProxy })} />
          </SettingRow>
        </SettingList>
      </Card>

      <Card>
        <CardHeader icon={<Link2 className="size-4" />} title="Connection details" description="Use these addresses in applications, browsers or other devices." />
        <div className="space-y-2 border-t border-line px-5 py-4">
          {listeners.filter((l) => l.enabled).length === 0 && <div className="text-[12.5px] text-faint">No enabled listeners.</div>}
          {listeners
            .filter((l) => l.enabled)
            .flatMap((l) => {
              const host = l.address === "0.0.0.0" || l.address === "::" ? "<this-computer-ip>" : l.address.includes(":") ? `[${l.address}]` : l.address;
              const schemes = l.type === "mixed" ? ["socks5h", "http"] : l.type === "socks" ? ["socks5h"] : ["http"];
              return schemes.map((scheme) => `${scheme}://${credentials}${host}:${l.port}`);
            })
            .map((url) => (
              <div key={url} className="flex items-center gap-2 rounded-lg border border-line bg-panel-2 py-1 pr-1 pl-3">
                <span className="selectable flex-1 truncate font-mono text-[12.5px]">{url}</span>
                <CopyButton text={url} />
              </div>
            ))}
        </div>
      </Card>
    </div>
  );
}

function UserRow({ user, error, onChange, onRemove }: { user: User; error?: string; onChange: (patch: Partial<User>) => void; onRemove: () => void }) {
  const [visible, setVisible] = useState(false);
  return (
    <div>
      <div className="flex items-center gap-2">
        <Input className="w-56" value={user.username} placeholder="Username" invalid={!!error} onChange={(event) => onChange({ username: event.target.value })} />
        <Input
          mono
          className="flex-1"
          type={visible ? "text" : "password"}
          value={user.password}
          placeholder="Password"
          onChange={(event) => onChange({ password: event.target.value })}
          trailing={
            <div className="flex">
              <IconButton label={visible ? "Hide" : "Show"} className="size-7" onClick={() => setVisible((v) => !v)}>
                {visible ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
              </IconButton>
            </div>
          }
        />
        <IconButton label="Generate password" onClick={() => onChange({ password: randomPassword() })}>
          <Wand2 className="size-4" />
        </IconButton>
        <CopyButton text={user.password} label="Copy password" />
        <IconButton label="Remove user" tone="danger" onClick={onRemove}>
          <Trash2 className="size-4" />
        </IconButton>
      </div>
      {error && <div className="mt-1 text-[12px] text-danger">{error}</div>}
    </div>
  );
}
