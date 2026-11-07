import { Globe, Lock, Router } from "lucide-react";
import { Callout, Card, CardHeader, Field, Input, PageHeader, Segmented, Select, SettingList, SettingRow } from "../components/ui";
import { useDraft } from "../lib/store";

const REMOTE_PRESETS = [
  { value: "https://1.1.1.1/dns-query", label: "Cloudflare · DNS over HTTPS" },
  { value: "tls://1.1.1.1", label: "Cloudflare · DNS over TLS" },
  { value: "udp://1.1.1.1", label: "Cloudflare · Plain (encrypted by WARP)" },
  { value: "https://8.8.8.8/dns-query", label: "Google · DNS over HTTPS" },
  { value: "https://9.9.9.9/dns-query", label: "Quad9 · DNS over HTTPS (malware blocking)" },
  { value: "https://dns.adguard-dns.com/dns-query", label: "AdGuard · DNS over HTTPS (ad blocking)" },
];

const DIRECT_PRESETS = [
  { value: "local", label: "System resolver" },
  { value: "https://1.1.1.1/dns-query", label: "Cloudflare · DNS over HTTPS" },
  { value: "udp://1.1.1.1", label: "Cloudflare · Plain" },
  { value: "udp://8.8.8.8", label: "Google · Plain" },
];

export function Dns() {
  const { draft, update, errors } = useDraft();
  const dns = draft.dns;
  const setDns = (patch: Partial<typeof dns>) => update((d) => ({ ...d, dns: { ...d.dns, ...patch } }));

  return (
    <div className="space-y-5 pb-20">
      <PageHeader
        title="DNS"
        description="How domain names are resolved. Harp uses separate resolvers for traffic sent through WARP and for traffic that bypasses it, so neither leaks into the other."
      />

      <Card>
        <CardHeader icon={<Lock className="size-4" />} title="Resolver through WARP" description="Used for domains routed through WARP. Queries travel inside the tunnel." />
        <div className="grid grid-cols-2 gap-4 border-t border-line px-5 py-4">
          <ResolverPicker presets={REMOTE_PRESETS} value={dns.remote} onChange={(remote) => setDns({ remote })} error={errors["dns.remote"]} />
        </div>
      </Card>

      <Card>
        <CardHeader icon={<Router className="size-4" />} title="Resolver for bypassed traffic" description="Used for domains that bypass WARP, so local services resolve to nearby servers." />
        <div className="grid grid-cols-2 gap-4 border-t border-line px-5 py-4">
          <ResolverPicker presets={DIRECT_PRESETS} value={dns.direct} onChange={(direct) => setDns({ direct })} error={errors["dns.direct"]} ipOnly />
        </div>
      </Card>

      <Card>
        <CardHeader icon={<Globe className="size-4" />} title="Address family" />
        <SettingList>
          <SettingRow title="Lookup strategy" description="Which address types to request when resolving names." error={errors["dns.strategy"]}>
            <Segmented
              size="sm"
              value={dns.strategy || "default"}
              onChange={(value) => setDns({ strategy: value === "default" ? "" : value })}
              options={[
                { value: "default", label: "Automatic" },
                { value: "prefer_ipv4", label: "Prefer IPv4" },
                { value: "prefer_ipv6", label: "Prefer IPv6" },
                { value: "ipv4_only", label: "IPv4 only" },
                { value: "ipv6_only", label: "IPv6 only" },
              ]}
            />
          </SettingRow>
        </SettingList>
      </Card>

      <Callout tone="accent" title="Supported formats">
        udp://1.1.1.1, tcp://1.1.1.1, tls://1.1.1.1, quic://dns.adguard-dns.com, https://1.1.1.1/dns-query and h3://1.1.1.1/dns-query.
        A plain IP address means UDP.
      </Callout>
    </div>
  );
}

function ResolverPicker({
  presets,
  value,
  onChange,
  error,
  ipOnly,
}: {
  presets: { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
  error?: string;
  ipOnly?: boolean;
}) {
  const preset = presets.some((p) => p.value === value) ? value : "custom";
  return (
    <>
      <Field label="Server">
        <Select value={preset} onChange={(event) => onChange(event.target.value === "custom" ? "" : event.target.value)}>
          {presets.map((p) => (
            <option key={p.value} value={p.value}>
              {p.label}
            </option>
          ))}
          <option value="custom">Custom…</option>
        </Select>
      </Field>
      <Field label="Address" error={error} description={ipOnly ? "Must use an IP address, or local for the system resolver." : undefined}>
        <Input mono value={value} disabled={preset !== "custom"} onChange={(event) => onChange(event.target.value.trim())} placeholder="https://1.1.1.1/dns-query" invalid={!!error} />
      </Field>
    </>
  );
}
