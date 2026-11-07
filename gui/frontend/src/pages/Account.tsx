import {
  ArrowRight,
  BadgeCheck,
  Download,
  FileDown,
  FileUp,
  KeyRound,
  Laptop,
  Pencil,
  RefreshCw,
  Server,
  ShieldCheck,
  Smartphone,
  Trash2,
  UserPlus,
  Wrench,
} from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { Logo } from "../components/Logo";
import {
  Badge,
  Button,
  Callout,
  Card,
  CardHeader,
  Checkbox,
  CopyButton,
  Field,
  IconButton,
  Input,
  PageHeader,
  SettingList,
  SettingRow,
  Spinner,
  Switch,
  cn,
  useFeedback,
} from "../components/ui";
import { api, errorMessage, isCancelled, type AccountInfo } from "../lib/api";
import { formatBytes, formatDate, maskSecret } from "../lib/format";
import { useDraft, useStore } from "../lib/store";

const TERMS_URL = "https://www.cloudflare.com/application/terms/";

function planName(account: AccountInfo): string {
  switch (account.type) {
    case "unlimited":
      return "WARP+ Unlimited";
    case "limited":
      return "WARP+";
    case "team":
      return "Zero Trust";
    case "free":
      return account.warpPlus ? "WARP+" : "WARP";
    default:
      return account.type || "WARP";
  }
}

export function Account() {
  const { state } = useStore();
  return (
    <div className="space-y-5 pb-20">
      <PageHeader title="WARP & account" description="Your Cloudflare WARP account, linked devices and how Harp connects to WARP." />
      {state.hasAccount ? (
        <>
          <AccountOverview />
          <ConnectionSettings />
          <Tools />
        </>
      ) : (
        <Register />
      )}
    </div>
  );
}

function Register() {
  const { toast } = useFeedback();
  const [accepted, setAccepted] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);

  const register = async () => {
    setBusy(true);
    try {
      await api.RegisterAccount(name.trim(), "PC");
      toast("success", "WARP account created", "You can connect now.");
    } catch (error) {
      toast("danger", "Registration failed", errorMessage(error));
    } finally {
      setBusy(false);
    }
  };

  const importAccount = async () => {
    try {
      if (await api.ImportAccount()) toast("success", "Account imported");
    } catch (error) {
      if (!isCancelled(error)) toast("danger", "Import failed", errorMessage(error));
    }
  };

  return (
    <div className="grid grid-cols-[minmax(0,1fr)_320px] gap-5">
      <Card className="p-7">
        <div className="flex items-center gap-4">
          <Logo size={52} />
          <div>
            <h2 className="text-[19px] font-semibold tracking-tight">Create a WARP account</h2>
            <p className="text-[13px] text-muted">Free, anonymous and ready in seconds.</p>
          </div>
        </div>
        <div className="mt-6 space-y-4">
          <Field label="Device name" description="Shown in the list of devices linked to your account. Leave empty for a random name.">
            <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="My laptop" />
          </Field>
          <Checkbox checked={accepted} onChange={setAccepted}>
            I agree to Cloudflare's{" "}
            <button type="button" className="font-medium text-accent hover:underline" onClick={() => api.OpenURL(TERMS_URL)}>
              Terms of Service
            </button>
            . Harp is not affiliated with Cloudflare.
          </Checkbox>
          <Button variant="primary" size="lg" icon={<UserPlus className="size-4" />} onClick={register} disabled={!accepted} loading={busy}>
            Create account
          </Button>
        </div>
      </Card>
      <Card className="p-6">
        <div className="flex size-9 items-center justify-center rounded-lg bg-accent-soft text-accent">
          <FileUp className="size-4" />
        </div>
        <h3 className="mt-4 text-[14px] font-semibold">Already have an account?</h3>
        <p className="mt-1 text-[12.5px] leading-relaxed text-muted">
          Import an account file created by the Harp or wgcf command line tools (<span className="font-mono">harp-account.toml</span>).
        </p>
        <Button className="mt-4" variant="secondary" icon={<FileUp className="size-4" />} onClick={importAccount}>
          Import account file
        </Button>
      </Card>
    </div>
  );
}

function AccountOverview() {
  const { toast, confirm } = useFeedback();
  const [account, setAccount] = useState<AccountInfo | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [license, setLicense] = useState("");
  const [applying, setApplying] = useState(false);
  const [renaming, setRenaming] = useState<string | null>(null);
  const [showLicense, setShowLicense] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setAccount(await api.GetAccount());
      setError("");
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const applyLicense = async () => {
    setApplying(true);
    try {
      await api.UpdateLicense(license);
      setLicense("");
      toast("success", "License key applied", "Reconnect to start using WARP+.");
      await load();
    } catch (e) {
      toast("danger", "Could not apply the license key", errorMessage(e));
    } finally {
      setApplying(false);
    }
  };

  const rename = async () => {
    if (renaming === null) return;
    try {
      await api.RenameDevice(renaming);
      setRenaming(null);
      await load();
    } catch (e) {
      toast("danger", "Could not rename the device", errorMessage(e));
    }
  };

  const setActive = async (id: string, active: boolean) => {
    try {
      await api.SetDeviceActive(id, active);
      await load();
    } catch (e) {
      toast("danger", "Could not update the device", errorMessage(e));
    }
  };

  const removeDevice = async (id: string, name: string) => {
    if (!(await confirm({ title: `Remove “${name || id}”?`, description: "The device is unlinked from your account and loses WARP+ access.", confirmLabel: "Remove device", danger: true }))) return;
    try {
      await api.RemoveDevice(id);
      await load();
    } catch (e) {
      toast("danger", "Could not remove the device", errorMessage(e));
    }
  };

  return (
    <>
      <Card>
        <CardHeader
          icon={<BadgeCheck className="size-4" />}
          title="Account"
          actions={
            <Button size="sm" variant="ghost" icon={<RefreshCw className={cn("size-3.5", loading && "animate-spin")} />} onClick={load} disabled={loading}>
              Refresh
            </Button>
          }
        />
        <div className="border-t border-line px-5 py-4">
          {loading && !account ? (
            <div className="flex items-center gap-2 text-[13px] text-muted">
              <Spinner /> Loading account details from Cloudflare…
            </div>
          ) : error && !account ? (
            <Callout tone="warning" title="Account details are unavailable">
              Cloudflare's API could not be reached ({error}). You can still connect; the WARP configuration is stored locally.
            </Callout>
          ) : account ? (
            <div className="grid grid-cols-4 gap-5">
              <Stat label="Plan">
                <span className="flex items-center gap-2">
                  {planName(account)}
                  {(account.warpPlus || account.type !== "free") && <Badge tone="accent">Premium</Badge>}
                </span>
              </Stat>
              <Stat label="WARP+ data">{formatBytes(account.premiumData)}</Stat>
              <Stat label="Quota">{account.quota ? formatBytes(account.quota) : "—"}</Stat>
              <Stat label="Role">{account.role || "—"}</Stat>
              <Stat label="This device" className="col-span-2">
                {renaming === null ? (
                  <span className="flex items-center gap-1.5">
                    {account.deviceName || "Unnamed"}
                    <IconButton label="Rename" className="size-6" onClick={() => setRenaming(account.deviceName)}>
                      <Pencil className="size-3" />
                    </IconButton>
                  </span>
                ) : (
                  <span className="flex items-center gap-2">
                    <Input className="w-56" value={renaming} autoFocus onChange={(event) => setRenaming(event.target.value)} onKeyDown={(event) => event.key === "Enter" && rename()} />
                    <Button size="sm" variant="primary" onClick={rename}>
                      Save
                    </Button>
                    <Button size="sm" variant="ghost" onClick={() => setRenaming(null)}>
                      Cancel
                    </Button>
                  </span>
                )}
              </Stat>
              <Stat label="License key" className="col-span-2">
                <span className="flex items-center gap-1">
                  <span className="selectable font-mono text-[12.5px]">{showLicense ? account.license : maskSecret(account.license)}</span>
                  <Button size="sm" variant="ghost" onClick={() => setShowLicense((v) => !v)}>
                    {showLicense ? "Hide" : "Show"}
                  </Button>
                  <CopyButton text={account.license} label="Copy license key" />
                </span>
              </Stat>
            </div>
          ) : null}
        </div>
      </Card>

      <Card>
        <CardHeader
          icon={<KeyRound className="size-4" />}
          title="Upgrade with a WARP+ license"
          description="Link this device to an existing WARP+ subscription. In the 1.1.1.1 mobile app: Menu → Account → Key."
        />
        <div className="border-t border-line px-5 py-4">
          <div className="flex gap-2">
            <Input mono className="flex-1" placeholder="xxxxxxxx-xxxxxxxx-xxxxxxxx" value={license} onChange={(event) => setLicense(event.target.value)} />
            <Button variant="primary" onClick={applyLicense} disabled={!license.trim()} loading={applying}>
              Apply key
            </Button>
          </div>
          <p className="mt-2 text-[12px] leading-relaxed text-faint">
            Up to 5 devices can share a subscription. Only keys from subscriptions bought in the 1.1.1.1 app are supported.
          </p>
        </div>
      </Card>

      {account && (account.devices ?? []).length > 0 && (
        <Card>
          <CardHeader icon={<Laptop className="size-4" />} title="Linked devices" description="Devices sharing this account." />
          <div className="divide-y divide-line border-t border-line">
            {(account.devices ?? []).map((device) => (
              <div key={device.id} className="flex items-center gap-3 px-5 py-3">
                <div className="flex size-8 items-center justify-center rounded-lg border border-line bg-panel-2 text-muted">
                  {/android|ios|iphone/i.test(device.type + device.model) ? <Smartphone className="size-4" /> : <Laptop className="size-4" />}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 text-[13px] font-medium">
                    {device.name || "Unnamed device"}
                    {device.current && <Badge tone="accent">This device</Badge>}
                    {!device.active && <Badge>Inactive</Badge>}
                  </div>
                  <div className="truncate text-[11.5px] text-faint">
                    {[device.model, device.type].filter(Boolean).join(" · ")} · Added {formatDate(device.created)}
                  </div>
                </div>
                {!device.current && (
                  <>
                    <Switch checked={device.active} onChange={(active) => setActive(device.id, active)} label="Active" />
                    <IconButton label="Remove device" tone="danger" onClick={() => removeDevice(device.id, device.name)}>
                      <Trash2 className="size-4" />
                    </IconButton>
                  </>
                )}
              </div>
            ))}
          </div>
        </Card>
      )}
    </>
  );
}

function Stat({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  return (
    <div className={className}>
      <div className="text-[11.5px] font-medium text-faint">{label}</div>
      <div className="mt-1 text-[13.5px] font-medium text-fg">{children}</div>
    </div>
  );
}

function ConnectionSettings() {
  const { draft, update, errors } = useDraft();
  const warp = draft.warp;
  const setWarp = (patch: Partial<typeof warp>) => update((d) => ({ ...d, warp: { ...d.warp, ...patch } }));
  const { setPage } = useStore();
  const custom = warp.endpoint !== "";

  return (
    <Card>
      <CardHeader icon={<Server className="size-4" />} title="Connection" description="How the WireGuard tunnel to Cloudflare is set up." />
      <SettingList>
        <SettingRow
          title="Endpoint"
          description={
            custom ? (
              <>
                Fixed: <span className="selectable font-mono">{warp.endpoint}</span>
              </>
            ) : (
              "Automatic: Harp picks an endpoint that works and switches when one gets blocked."
            )
          }
        >
          <Button size="sm" variant="secondary" icon={<ArrowRight className="size-3.5" />} onClick={() => setPage("endpoints")}>
            Manage endpoints
          </Button>
        </SettingRow>
        <SettingRow title="Tunnel MTU" description="Lower it (e.g. 1280 → 1200) if some sites stall; raise it for speed on good networks." error={errors["warp.mtu"]}>
          <Input mono type="number" className="w-28" value={warp.mtu || ""} onChange={(event) => setWarp({ mtu: Number(event.target.value) || 0 })} />
        </SettingRow>
        <SettingRow title="Keepalive" description="Seconds between keepalive packets, keeping NAT mappings open. 0 disables." error={errors["warp.keepalive"]}>
          <Input mono type="number" className="w-28" value={warp.keepalive} onChange={(event) => setWarp({ keepalive: Number(event.target.value) || 0 })} />
        </SettingRow>
        <SettingRow title="Send client identifier" description="Include Cloudflare's reserved bytes in WireGuard packets, as the official app does. Recommended.">
          <Switch checked={warp.reserved} onChange={(reserved) => setWarp({ reserved })} />
        </SettingRow>
      </SettingList>
    </Card>
  );
}

function Tools() {
  const { toast, confirm } = useFeedback();
  const [refreshing, setRefreshing] = useState(false);

  const run = async (action: () => Promise<string>, title: string) => {
    try {
      const path = await action();
      if (path) toast("success", title, path);
    } catch (error) {
      if (!isCancelled(error)) toast("danger", "Export failed", errorMessage(error));
    }
  };

  const refresh = async () => {
    setRefreshing(true);
    try {
      await api.RefreshPeerConfig();
      toast("success", "Peer configuration refreshed", "Reconnect to use it.");
    } catch (error) {
      toast("danger", "Refresh failed", errorMessage(error));
    } finally {
      setRefreshing(false);
    }
  };

  const remove = async () => {
    const ok = await confirm({
      title: "Remove this account from Harp?",
      description: "Harp disconnects and deletes the local account file (a .bak copy is kept). The account itself is not deleted at Cloudflare.",
      confirmLabel: "Remove account",
      danger: true,
    });
    if (!ok) return;
    try {
      await api.RemoveAccount();
      toast("success", "Account removed");
    } catch (error) {
      toast("danger", "Could not remove the account", errorMessage(error));
    }
  };

  return (
    <Card>
      <CardHeader icon={<Wrench className="size-4" />} title="Tools" />
      <SettingList>
        <SettingRow title="Refresh peer configuration" description="Download the WireGuard settings for this device from Cloudflare again.">
          <Button variant="secondary" size="sm" icon={<RefreshCw className="size-3.5" />} loading={refreshing} onClick={refresh}>
            Refresh
          </Button>
        </SettingRow>
        <SettingRow title="WireGuard profile" description="Use WARP with the official WireGuard app or a router.">
          <Button variant="secondary" size="sm" icon={<FileDown className="size-3.5" />} onClick={() => run(api.ExportWireGuardProfile, "WireGuard profile saved")}>
            Export .conf
          </Button>
        </SettingRow>
        <SettingRow title="sing-box configuration" description="The complete configuration Harp runs, for use with “harp run” or sing-box. Contains your private key.">
          <Button variant="secondary" size="sm" icon={<Download className="size-3.5" />} onClick={() => run(api.ExportSingBoxConfig, "Configuration saved")}>
            Export .json
          </Button>
        </SettingRow>
        <SettingRow title={<span className="text-danger">Remove account</span>} description="Unlink this account from Harp on this computer.">
          <Button variant="danger" size="sm" icon={<Trash2 className="size-3.5" />} onClick={remove}>
            Remove
          </Button>
        </SettingRow>
      </SettingList>
      <div className="flex items-center gap-2 border-t border-line px-5 py-3 text-[12px] text-faint">
        <ShieldCheck className="size-3.5" /> Your keys never leave this computer except to register with Cloudflare.
      </div>
    </Card>
  );
}
