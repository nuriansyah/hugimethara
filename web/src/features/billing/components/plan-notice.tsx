import { Sparkles } from "lucide-react";
import Link from "next/link";
import { Alert, AlertDescription, AlertTitle } from "@hugimethara/ui/components/alert";

/** Explains that something needs a higher plan and where to see plan details. */
export function PlanNotice({ title = "Upgrade required", message }: { title?: string; message: string }) {
  return (
    <Alert>
      <Sparkles aria-hidden />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        <p>{message}</p>
        <Link href="/settings/plan" className="font-medium underline underline-offset-4">
          Compare plans
        </Link>
      </AlertDescription>
    </Alert>
  );
}
