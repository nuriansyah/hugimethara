import type { FieldValues, Path, UseFormSetError } from "react-hook-form";
import { ApiError } from "@/lib/api/client";

/**
 * Maps API field errors onto form fields. Returns the message to show at form
 * level when the error is not (only) about specific fields.
 */
export function applyApiError<T extends FieldValues>(
  err: unknown,
  setError: UseFormSetError<T>,
  fields: readonly Path<T>[],
): string | undefined {
  if (!(err instanceof ApiError)) return "The server could not be reached. Check your connection and try again.";
  let unmatched = false;
  for (const fe of err.fieldErrors ?? []) {
    const name = fields.find((f) => f === fe.field);
    if (name) setError(name, { type: fe.code, message: fe.message });
    else unmatched = true;
  }
  if (!err.fieldErrors?.length || unmatched) {
    return err.requestId ? `${err.message} (request ${err.requestId})` : err.message;
  }
  return undefined;
}

/** Only same-site relative paths are accepted as post-login destinations. */
export function safeNext(next: string | null | undefined): string {
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.startsWith("/\\")) return "/";
  return next;
}

/**
 * For dialogs that contain forms: ignore clicks and focus moves outside the
 * dialog so typed input is never lost by accident (Escape and Cancel still close).
 */
export function keepOpen(e: Event) {
  e.preventDefault();
}
