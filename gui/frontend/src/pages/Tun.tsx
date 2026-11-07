import { Cpu, MonitorSmartphone, Plus, ShieldCheck, Split, X } from "lucide-react";
import { useState } from "react";
import {
  Badge,
  Button,
  Callout,
  Card,
  CardHeader,
  IconButton,
  Input,
  Kbd,
  PageHeader,
  Segmented,
  SettingList,
  SettingRow,
  Switch,
} from "../components/ui";
import { api } from "../lib/api";
import { useDraft, useStore } from "../lib/store";

const STACKS = {
  mixed: "System stack for TCP and the user-space stack for UDP. Best compatibility; recommended.",
  system: "Uses the operating system's network stack. Fastest, but some firewalls interfere.",
  gvisor: "Fully user-space network stack. Most isolated, slightly higher CPU usage.",
};

export function Tun() {
  const { draft, update, errors } = useDraft();
  const { info } = useStore();
  const tun = draft.tun;
  const setTun = (patch: Partial<typeof tun>) => update((d) => ({ ...d, tun: { ...d.tun, ...patch } }));
  const linux = info?.os === "linux";

  return (
    <div className="space-y-5 pb-20">
      <PageHeader
        title="TUN mode"
        description="Creates a virtual network adapter so every application on this device uses Harp, without configuring proxies. Your routing rules still decide where each connection goes."
      />

      {info &&
        (info.elevated ? (
          <Callout tone="success" title="Running with administrator rights">
            TUN mode can be used.
          </Callout>
        ) : (
          <Callout
            tone="warning"
            title="TUN mode needs administrator rights"
            action={
              info.canRelaunch ? (
                <Button size="sm" variant="primary" onClick={() => api.RelaunchAsAdmin(false)}>
                  Restart as administrator
                </Button>
              ) : undefined
            }
          >
            {info.canRelaunch ? (
              "Harp will ask for permission and restart. Proxy mode works without it."
            ) : (
              <>
                To use TUN mode, <Kbd>{info.elevationHint}</Kbd>
              </>
            )}
          </Callout>
        ))}

      <Card>
        <CardHeader icon={<Cpu className="size-4" />} title="Network stack" />
        <SettingList>
          <SettingRow title="Stack" description={STACKS[tun.stack as keyof typeof STACKS]} error={errors["tun.stack"]}>
            <Segmented
              value={tun.stack}
              onChange={(stack) => setTun({ stack })}
              options={[
                { value: "mixed", label: "Mixed" },
                { value: "system", label: "System" },
                { value: "gvisor", label: "gVisor" },
              ]}
            />
          </SettingRow>
          <SettingRow title="MTU" description="Maximum packet size of the virtual adapter. The default of 9000 suits almost every network." error={errors["tun.mtu"]}>
            <Input mono type="number" className="w-28" value={tun.mtu || ""} onChange={(event) => setTun({ mtu: Number(event.target.value) || 0 })} />
          </SettingRow>
        </SettingList>
      </Card>

      <Card>
        <CardHeader icon={<ShieldCheck className="size-4" />} title="Leak protection & integration" />
        <SettingList>
          <SettingRow
            title="Strict routing"
            description="Prevent traffic and DNS queries from escaping the tunnel through other adapters. Recommended."
          >
            <Switch checked={tun.strictRoute} onChange={(strictRoute) => setTun({ strictRoute })} />
          </SettingRow>
          <SettingRow
            title={
              <span className="flex items-center gap-2">
                Auto redirect <Badge>Linux</Badge>
              </span>
            }
            description="Use nftables redirection for TCP. Improves performance and compatibility with Docker on Linux."
            disabled={!linux}
          >
            <Switch checked={tun.autoRedirect} disabled={!linux} onChange={(autoRedirect) => setTun({ autoRedirect })} />
          </SettingRow>
          <SettingRow title="Also run proxy servers" description="Keep the listeners from the Proxy page running in TUN mode, e.g. to share WARP with other devices.">
            <Switch checked={tun.serveProxy} onChange={(serveProxy) => setTun({ serveProxy })} />
          </SettingRow>
        </SettingList>
      </Card>

      <Card>
        <CardHeader
          icon={<Split className="size-4" />}
          title="Route table"
          description="Decide at the operating system level which networks enter the tunnel. For most cases, routing rules are more flexible."
        />
        <div className="grid grid-cols-2 gap-5 border-t border-line px-5 py-4">
          <CidrList
            title="Excluded networks"
            description="Never sent into the tunnel, e.g. 192.168.0.0/16."
            values={tun.excludeRoutes ?? []}
            error={errors["tun.excludeRoutes"]}
            onChange={(excludeRoutes) => setTun({ excludeRoutes })}
          />
          <CidrList
            title="Included networks only"
            description="When set, only these networks enter the tunnel."
            values={tun.includeRoutes ?? []}
            error={errors["tun.includeRoutes"]}
            onChange={(includeRoutes) => setTun({ includeRoutes })}
          />
        </div>
      </Card>

      <Callout tone="accent" title="Per-application routing">
        <span className="inline-flex items-center gap-1">
          <MonitorSmartphone className="inline size-3.5" /> Add “Application” rules on the Routing rules page to send specific programs through WARP or around it.
        </span>
      </Callout>
    </div>
  );
}

function CidrList({
  title,
  description,
  values,
  error,
  onChange,
}: {
  title: string;
  description: string;
  values: string[];
  error?: string;
  onChange: (values: string[]) => void;
}) {
  const [input, setInput] = useState("");
  const add = () => {
    const additions = input
      .split(/[\s,]+/)
      .map((v) => v.trim())
      .filter(Boolean)
      .map((v) => (v.includes("/") ? v : v.includes(":") ? `${v}/128` : `${v}/32`))
      .filter((v) => !values.includes(v));
    if (additions.length) onChange([...values, ...additions]);
    setInput("");
  };
  return (
    <div>
      <div className="text-[13px] font-medium">{title}</div>
      <div className="mb-2.5 text-[12px] text-muted">{description}</div>
      <div className="flex gap-2">
        <Input mono className="flex-1" placeholder="10.0.0.0/8" value={input} onChange={(event) => setInput(event.target.value)} onKeyDown={(event) => event.key === "Enter" && add()} />
        <Button variant="secondary" icon={<Plus className="size-4" />} onClick={add} disabled={!input.trim()}>
          Add
        </Button>
      </div>
      {error && <div className="mt-1.5 text-[12px] text-danger">{error}</div>}
      <div className="mt-2.5 flex flex-wrap gap-1.5">
        {values.map((value) => (
          <span key={value} className="inline-flex h-7 items-center gap-1 rounded-lg border border-line bg-panel-2 pr-0.5 pl-2.5 font-mono text-[12px]">
            {value}
            <IconButton label="Remove" className="size-6" onClick={() => onChange(values.filter((v) => v !== value))}>
              <X className="size-3" />
            </IconButton>
          </span>
        ))}
        {values.length === 0 && <span className="text-[12px] text-faint">None</span>}
      </div>
    </div>
  );
}
