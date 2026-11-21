"use client";

import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { Button } from "@hugimethara/ui/components/button";
import { Input } from "@hugimethara/ui/components/input";
import { Label } from "@hugimethara/ui/components/label";

/** A read-only value with a copy button (for links and secrets shown once). */
export function CopyField({ id, label, value }: { id: string; label: string; value: string }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      document.getElementById(id)?.focus();
      (document.getElementById(id) as HTMLInputElement | null)?.select();
    }
  }
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex gap-2">
        <Input id={id} readOnly value={value} className="font-mono text-xs" onFocus={(e) => e.currentTarget.select()} />
        <Button type="button" variant="outline" size="icon" onClick={copy} aria-label={copied ? "Copied" : `Copy ${label.toLowerCase()}`}>
          {copied ? <Check /> : <Copy />}
        </Button>
      </div>
    </div>
  );
}
