import clsx from "clsx";
import { AlertTriangle, Check, CheckCircle2, Copy, Info, Loader2, X, XCircle } from "lucide-react";
import {
  createContext,
  forwardRef,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from "react";
import { api } from "../lib/api";

export const cn = clsx;

// ---------------------------------------------------------------- Button

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger" | "subtle";
type ButtonSize = "sm" | "md" | "lg";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  icon?: ReactNode;
}

const buttonVariants: Record<ButtonVariant, string> = {
  primary:
    "bg-accent text-white shadow-[0_1px_0_rgba(255,255,255,0.15)_inset,0_6px_16px_-6px_var(--accent)] hover:brightness-110 active:brightness-95",
  secondary: "bg-panel-2 text-fg border border-line hover:bg-hover hover:border-line-strong",
  ghost: "text-muted hover:text-fg hover:bg-hover",
  danger: "bg-danger-soft text-danger border border-transparent hover:border-danger/40",
  subtle: "bg-accent-soft text-accent hover:brightness-110",
};

const buttonSizes: Record<ButtonSize, string> = {
  sm: "h-7 px-2.5 text-[12.5px] gap-1.5 rounded-lg",
  md: "h-9 px-3.5 text-[13px] gap-2 rounded-[10px]",
  lg: "h-11 px-5 text-sm gap-2 rounded-xl",
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "secondary", size = "md", loading, icon, className, children, disabled, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={cn(
        "inline-flex select-none items-center justify-center whitespace-nowrap font-medium transition-all duration-150 disabled:pointer-events-none disabled:opacity-50",
        buttonVariants[variant],
        buttonSizes[size],
        className,
      )}
      {...props}
    >
      {loading ? <Loader2 className="size-4 animate-spin" /> : icon}
      {children}
    </button>
  );
});

export function IconButton({
  label,
  className,
  children,
  tone = "default",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string; tone?: "default" | "danger" }) {
  return (
    <button
      title={label}
      aria-label={label}
      className={cn(
        "inline-flex size-8 items-center justify-center rounded-lg text-muted transition-colors disabled:pointer-events-none disabled:opacity-40",
        tone === "danger" ? "hover:bg-danger-soft hover:text-danger" : "hover:bg-hover hover:text-fg",
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}

// ---------------------------------------------------------------- Layout

export function Card({ className, children }: { className?: string; children: ReactNode }) {
  return <section className={cn("card", className)}>{children}</section>;
}

export function CardHeader({
  title,
  description,
  icon,
  actions,
  className,
}: {
  title: ReactNode;
  description?: ReactNode;
  icon?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex items-start gap-3 px-5 pt-4 pb-3", className)}>
      {icon && (
        <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg bg-accent-soft text-accent">
          {icon}
        </div>
      )}
      <div className="min-w-0 flex-1">
        <h3 className="text-[14px] font-semibold tracking-tight text-fg">{title}</h3>
        {description && <p className="mt-0.5 text-[12.5px] leading-relaxed text-muted">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="flex items-end justify-between gap-6 pb-5">
      <div className="min-w-0">
        <h1 className="text-[22px] font-semibold tracking-tight text-fg">{title}</h1>
        {description && <p className="mt-1 max-w-2xl text-[13px] leading-relaxed text-muted">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </header>
  );
}

export function Divider({ className }: { className?: string }) {
  return <div className={cn("h-px bg-line", className)} />;
}

// ---------------------------------------------------------------- Form controls

export function Switch({
  checked,
  onChange,
  disabled,
  label,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  label?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative inline-flex h-[22px] w-[38px] shrink-0 items-center rounded-full border transition-colors duration-200 disabled:cursor-not-allowed disabled:opacity-50",
        checked ? "border-transparent bg-accent" : "border-line-strong bg-panel-2",
      )}
    >
      <span
        className={cn(
          "inline-block size-4 rounded-full bg-white shadow-sm transition-transform duration-200",
          checked ? "translate-x-[18px]" : "translate-x-[2px]",
        )}
      />
    </button>
  );
}

export function Checkbox({
  checked,
  onChange,
  children,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  children: ReactNode;
}) {
  return (
    <label className="flex cursor-pointer items-start gap-2.5 text-[13px] text-muted">
      <button
        type="button"
        role="checkbox"
        aria-checked={checked}
        onClick={() => onChange(!checked)}
        className={cn(
          "mt-0.5 flex size-4 shrink-0 items-center justify-center rounded border transition-colors",
          checked ? "border-accent bg-accent text-white" : "border-line-strong bg-panel-2",
        )}
      >
        {checked && <Check className="size-3" strokeWidth={3} />}
      </button>
      <span className="leading-snug">{children}</span>
    </label>
  );
}

const fieldBase =
  "w-full rounded-[10px] border bg-panel-2 text-[13px] text-fg placeholder:text-faint transition-colors focus:outline-none focus:border-accent focus:ring-3 focus:ring-accent-soft disabled:opacity-50";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean;
  leading?: ReactNode;
  trailing?: ReactNode;
  mono?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { invalid, leading, trailing, mono, className, ...props },
  ref,
) {
  return (
    <div className={cn("relative flex items-center", className)}>
      {leading && <div className="pointer-events-none absolute left-3 flex text-faint">{leading}</div>}
      <input
        ref={ref}
        spellCheck={false}
        autoComplete="off"
        className={cn(
          fieldBase,
          "h-9",
          leading ? "pl-9" : "pl-3",
          trailing ? "pr-10" : "pr-3",
          mono && "font-mono text-[12.5px]",
          invalid ? "border-danger" : "border-line",
        )}
        {...props}
      />
      {trailing && <div className="absolute right-1.5 flex items-center">{trailing}</div>}
    </div>
  );
});

export function TextArea({
  invalid,
  mono,
  className,
  ...props
}: TextareaHTMLAttributes<HTMLTextAreaElement> & { invalid?: boolean; mono?: boolean }) {
  return (
    <textarea
      spellCheck={false}
      className={cn(
        fieldBase,
        "min-h-20 px-3 py-2 leading-relaxed",
        mono && "font-mono text-[12.5px]",
        invalid ? "border-danger" : "border-line",
        className,
      )}
      {...props}
    />
  );
}

export function Select({
  className,
  children,
  invalid,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement> & { invalid?: boolean }) {
  return (
    <div className={cn("relative", className)}>
      <select
        className={cn(
          fieldBase,
          "h-9 cursor-pointer appearance-none pr-9 pl-3",
          invalid ? "border-danger" : "border-line",
        )}
        {...props}
      >
        {children}
      </select>
      <svg
        className="pointer-events-none absolute top-1/2 right-3 size-3.5 -translate-y-1/2 text-faint"
        viewBox="0 0 16 16"
        fill="none"
      >
        <path d="M4 6l4 4 4-4" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </div>
  );
}

export interface SegmentOption<T extends string> {
  value: T;
  label: ReactNode;
  icon?: ReactNode;
  disabled?: boolean;
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
  size = "md",
  className,
}: {
  value: T;
  options: SegmentOption<T>[];
  onChange: (value: T) => void;
  size?: "sm" | "md";
  className?: string;
}) {
  return (
    <div className={cn("inline-flex rounded-[11px] border border-line bg-panel-2 p-[3px]", className)}>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          disabled={option.disabled}
          onClick={() => onChange(option.value)}
          className={cn(
            "inline-flex flex-1 items-center justify-center gap-1.5 rounded-lg font-medium whitespace-nowrap transition-all duration-150 disabled:opacity-40",
            size === "sm" ? "h-7 px-2.5 text-[12px]" : "h-8 px-3.5 text-[13px]",
            value === option.value ? "bg-panel text-fg shadow-card ring-1 ring-line" : "text-muted hover:text-fg",
          )}
        >
          {option.icon}
          {option.label}
        </button>
      ))}
    </div>
  );
}

export function Field({
  label,
  description,
  error,
  children,
  className,
}: {
  label?: ReactNode;
  description?: ReactNode;
  error?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      {label && <label className="text-[12.5px] font-medium text-fg">{label}</label>}
      {children}
      {error ? (
        <p className="text-[12px] text-danger">{error}</p>
      ) : (
        description && <p className="text-[12px] leading-relaxed text-faint">{description}</p>
      )}
    </div>
  );
}

/** A labelled row with a control on the right, used in settings lists. */
export function SettingRow({
  title,
  description,
  children,
  error,
  disabled,
}: {
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  error?: string;
  disabled?: boolean;
}) {
  return (
    <div className={cn("flex items-center gap-6 px-5 py-3.5", disabled && "opacity-60")}>
      <div className="min-w-0 flex-1">
        <div className="text-[13px] font-medium text-fg">{title}</div>
        {description && <div className="mt-0.5 text-[12.5px] leading-relaxed text-muted">{description}</div>}
        {error && <div className="mt-1 text-[12px] text-danger">{error}</div>}
      </div>
      <div className="flex shrink-0 items-center">{children}</div>
    </div>
  );
}

export function SettingList({ children }: { children: ReactNode }) {
  return <div className="divide-y divide-line border-t border-line">{children}</div>;
}

// ---------------------------------------------------------------- Display

type Tone = "neutral" | "accent" | "success" | "warning" | "danger" | "cyan";

const badgeTones: Record<Tone, string> = {
  neutral: "bg-panel-2 text-muted border-line",
  accent: "bg-accent-soft text-accent border-transparent",
  success: "bg-success-soft text-success border-transparent",
  warning: "bg-warning-soft text-warning border-transparent",
  danger: "bg-danger-soft text-danger border-transparent",
  cyan: "bg-cyan/10 text-cyan border-transparent",
};

export function Badge({
  tone = "neutral",
  children,
  className,
  dot,
}: {
  tone?: Tone;
  children: ReactNode;
  className?: string;
  dot?: boolean;
}) {
  return (
    <span
      className={cn(
        "inline-flex h-[22px] items-center gap-1.5 rounded-md border px-2 text-[11.5px] font-medium whitespace-nowrap",
        badgeTones[tone],
        className,
      )}
    >
      {dot && <span className="size-1.5 rounded-full bg-current" />}
      {children}
    </span>
  );
}

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn("size-4 animate-spin text-muted", className)} />;
}

const calloutTones: Record<Exclude<Tone, "neutral" | "cyan">, { box: string; icon: ReactNode }> = {
  accent: { box: "border-accent/25 bg-accent-soft", icon: <Info className="size-4 text-accent" /> },
  success: { box: "border-success/25 bg-success-soft", icon: <CheckCircle2 className="size-4 text-success" /> },
  warning: { box: "border-warning/30 bg-warning-soft", icon: <AlertTriangle className="size-4 text-warning" /> },
  danger: { box: "border-danger/30 bg-danger-soft", icon: <XCircle className="size-4 text-danger" /> },
};

export function Callout({
  tone = "accent",
  title,
  children,
  action,
  stacked,
  className,
}: {
  tone?: keyof typeof calloutTones;
  title?: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  /** Places the action below the text, for narrow containers. */
  stacked?: boolean;
  className?: string;
}) {
  const style = calloutTones[tone];
  return (
    <div className={cn("flex items-start gap-3 rounded-xl border px-4 py-3", style.box, className)}>
      <div className="mt-0.5 shrink-0">{style.icon}</div>
      <div className="min-w-0 flex-1 text-[12.5px] leading-relaxed">
        {title && <div className="font-semibold text-fg">{title}</div>}
        {children && <div className="text-muted">{children}</div>}
        {action && stacked && <div className="mt-2.5">{action}</div>}
      </div>
      {action && !stacked && <div className="shrink-0 self-center">{action}</div>}
    </div>
  );
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center justify-center px-6 py-14 text-center", className)}>
      <div className="mb-4 flex size-12 items-center justify-center rounded-2xl border border-line bg-panel-2 text-muted">
        {icon}
      </div>
      <h3 className="text-[14px] font-semibold text-fg">{title}</h3>
      {description && <p className="mt-1 max-w-sm text-[12.5px] leading-relaxed text-muted">{description}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}

export function Kbd({ children }: { children: ReactNode }) {
  return (
    <code className="selectable rounded-md border border-line bg-panel-2 px-1.5 py-0.5 font-mono text-[11.5px] text-fg">
      {children}
    </code>
  );
}

export function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number>(0);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  return (
    <IconButton
      label={copied ? "Copied" : label}
      onClick={async () => {
        await api.CopyText(text);
        setCopied(true);
        window.clearTimeout(timer.current);
        timer.current = window.setTimeout(() => setCopied(false), 1400);
      }}
    >
      {copied ? <Check className="size-4 text-success" /> : <Copy className="size-[15px]" />}
    </IconButton>
  );
}

// ---------------------------------------------------------------- Modal

export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  width = "md",
}: {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  width?: "sm" | "md" | "lg";
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-6">
      <div className="animate-fade-in absolute inset-0 bg-black/50 backdrop-blur-[2px]" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        className={cn(
          "card animate-pop relative flex max-h-[calc(100vh-48px)] w-full flex-col overflow-hidden",
          width === "sm" && "max-w-md",
          width === "md" && "max-w-xl",
          width === "lg" && "max-w-3xl",
        )}
      >
        <div className="flex items-start gap-3 border-b border-line px-5 py-4">
          <div className="min-w-0 flex-1">
            <h2 className="text-[15px] font-semibold tracking-tight">{title}</h2>
            {description && <p className="mt-0.5 text-[12.5px] leading-relaxed text-muted">{description}</p>}
          </div>
          <IconButton label="Close" onClick={onClose} className="-mt-1 -mr-2">
            <X className="size-4" />
          </IconButton>
        </div>
        {children && <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">{children}</div>}
        {footer && (
          <div className="flex items-center justify-end gap-2 border-t border-line bg-panel-2/60 px-5 py-3">{footer}</div>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- Toasts

type ToastTone = "success" | "danger" | "accent" | "warning";

interface Toast {
  id: number;
  tone: ToastTone;
  title: string;
  description?: string;
}

interface Feedback {
  toast: (tone: ToastTone, title: string, description?: string) => void;
  confirm: (options: ConfirmOptions) => Promise<boolean>;
}

interface ConfirmOptions {
  title: string;
  description?: ReactNode;
  confirmLabel?: string;
  danger?: boolean;
}

const FeedbackContext = createContext<Feedback | null>(null);

const toastIcons: Record<ToastTone, ReactNode> = {
  success: <CheckCircle2 className="size-4 text-success" />,
  danger: <XCircle className="size-4 text-danger" />,
  accent: <Info className="size-4 text-accent" />,
  warning: <AlertTriangle className="size-4 text-warning" />,
};

export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [pending, setPending] = useState<(ConfirmOptions & { resolve: (value: boolean) => void }) | null>(null);
  const nextId = useRef(1);

  const dismiss = useCallback((id: number) => setToasts((all) => all.filter((t) => t.id !== id)), []);

  const toast = useCallback(
    (tone: ToastTone, title: string, description?: string) => {
      const id = nextId.current++;
      setToasts((all) => [...all.slice(-3), { id, tone, title, description }]);
      window.setTimeout(() => dismiss(id), tone === "danger" ? 7000 : 4000);
    },
    [dismiss],
  );

  const confirm = useCallback(
    (options: ConfirmOptions) => new Promise<boolean>((resolve) => setPending({ ...options, resolve })),
    [],
  );

  const close = (value: boolean) => {
    pending?.resolve(value);
    setPending(null);
  };

  return (
    <FeedbackContext.Provider value={{ toast, confirm }}>
      {children}
      <div className="pointer-events-none fixed right-5 bottom-5 z-[60] flex w-[360px] flex-col gap-2">
        {toasts.map((t) => (
          <div key={t.id} className="card animate-pop pointer-events-auto flex items-start gap-3 px-4 py-3">
            <div className="mt-0.5 shrink-0">{toastIcons[t.tone]}</div>
            <div className="min-w-0 flex-1">
              <div className="text-[13px] font-medium text-fg">{t.title}</div>
              {t.description && (
                <div className="selectable mt-0.5 text-[12px] leading-relaxed break-words text-muted">{t.description}</div>
              )}
            </div>
            <IconButton label="Dismiss" className="-mt-1 -mr-2 size-6" onClick={() => dismiss(t.id)}>
              <X className="size-3.5" />
            </IconButton>
          </div>
        ))}
      </div>
      <Modal
        open={!!pending}
        onClose={() => close(false)}
        title={pending?.title}
        width="sm"
        footer={
          <>
            <Button variant="ghost" onClick={() => close(false)}>
              Cancel
            </Button>
            <Button variant={pending?.danger ? "danger" : "primary"} onClick={() => close(true)} autoFocus>
              {pending?.confirmLabel ?? "Confirm"}
            </Button>
          </>
        }
      >
        {pending?.description && <div className="text-[13px] leading-relaxed text-muted">{pending.description}</div>}
      </Modal>
    </FeedbackContext.Provider>
  );
}

export function useFeedback(): Feedback {
  const value = useContext(FeedbackContext);
  if (!value) throw new Error("useFeedback outside FeedbackProvider");
  return value;
}
