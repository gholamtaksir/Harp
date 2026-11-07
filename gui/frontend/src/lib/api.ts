export { HarpService as api } from "../../bindings/github.com/gholamtaksir/Harp/gui";
export type { AppInfo, SaveResult } from "../../bindings/github.com/gholamtaksir/Harp/gui";
export type {
  AccountInfo,
  Connection,
  Device,
  EndpointResult,
  FieldError,
  Listener,
  LogEntry,
  Process,
  Rule,
  ScanRequest,
  ScanState,
  Settings,
  State,
  Stats,
  Trace,
  User,
} from "../../bindings/github.com/gholamtaksir/Harp/core/models";
export { HarpLogs, HarpScan, HarpSettings, HarpState, HarpStats } from "../../bindings/events";

/** Extracts a readable message from a rejected backend call. */
export function errorMessage(error: unknown): string {
  if (error == null) return "Unknown error";
  if (typeof error === "string") return error;
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) return String((error as { message: unknown }).message);
  return String(error);
}

/** True when a backend call was cancelled rather than failed. */
export function isCancelled(error: unknown): boolean {
  return errorMessage(error).toLowerCase().includes("cancel");
}
