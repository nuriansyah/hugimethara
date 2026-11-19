import { toast } from "sonner";
import { ApiError } from "@/lib/api/client";

/** Human message for any failed mutation (plan errors included). */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.requestId && err.status >= 500 ? `${err.message} (request ${err.requestId})` : err.message;
  return "The server could not be reached. Check your connection and try again.";
}

export function toastError(err: unknown) {
  toast.error(errorMessage(err));
}
