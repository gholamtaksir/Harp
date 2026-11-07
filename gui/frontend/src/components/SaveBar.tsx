import { RotateCcw, Save } from "lucide-react";
import { useStore } from "../lib/store";
import { Button, useFeedback } from "./ui";

/** Floating bar shown while settings have unsaved changes. */
export function SaveBar() {
  const { dirty, saving, save, discard, state } = useStore();
  const { toast } = useFeedback();
  if (!dirty) return null;
  const connected = state.status === "connected";

  const onSave = async () => {
    const result = await save();
    if (result.ok) {
      toast("success", connected ? "Settings applied" : "Settings saved", connected ? "Harp reconnected with the new settings." : undefined);
    } else {
      toast("danger", "Settings were not saved", result.error);
    }
  };

  return (
    <div className="pointer-events-none absolute inset-x-0 bottom-5 z-40 flex justify-center">
      <div className="card animate-pop pointer-events-auto flex items-center gap-4 py-2.5 pr-2.5 pl-4">
        <div className="flex items-center gap-2.5">
          <span className="size-2 rounded-full bg-warning" />
          <div>
            <div className="text-[13px] font-medium">Unsaved changes</div>
            {connected && <div className="text-[11.5px] text-faint">Saving will briefly reconnect</div>}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" icon={<RotateCcw className="size-3.5" />} onClick={discard} disabled={saving}>
            Discard
          </Button>
          <Button variant="primary" size="sm" icon={<Save className="size-3.5" />} onClick={onSave} loading={saving}>
            Save changes
          </Button>
        </div>
      </div>
    </div>
  );
}
