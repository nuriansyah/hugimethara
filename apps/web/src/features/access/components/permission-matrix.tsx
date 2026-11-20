"use client";

import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import type { ResourceType } from "../api";

/**
 * Checkbox grid over the permission catalog. Values are permission patterns:
 * "type:action", "type:*" (every action of a type) or "*:*" (full access).
 * Patterns the grid cannot express (e.g. "*:read") are kept untouched.
 */
export function PermissionMatrix({
  types,
  value,
  onChange,
  disabled,
}: {
  types: ResourceType[];
  value: string[];
  onChange: (next: string[]) => void;
  disabled?: boolean;
}) {
  const set = new Set(value);
  const full = set.has("*:*");
  const toggle = (perm: string, on: boolean) => {
    const next = new Set(set);
    if (on) next.add(perm);
    else next.delete(perm);
    onChange([...next].sort());
  };
  const setType = (type: string, on: boolean) => {
    const next = new Set([...set].filter((p) => !p.startsWith(type + ":")));
    if (on) next.add(type + ":*");
    onChange([...next].sort());
  };
  const exotic = value.filter((p) => p !== "*:*" && p.startsWith("*:"));

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <Checkbox id="perm-full" checked={full} disabled={disabled} onCheckedChange={(c) => toggle("*:*", c === true)} />
        <Label htmlFor="perm-full">Full access (every permission, including future ones)</Label>
      </div>
      <div className="max-h-80 overflow-y-auto rounded-md border">
        <table className="w-full text-sm">
          <tbody>
            {types.map((t) => {
              const all = full || set.has(t.type + ":*");
              return (
                <tr key={t.type} className="border-b last:border-0 align-top">
                  <th scope="row" className="w-44 p-2 text-left font-medium">
                    <div className="flex items-center gap-2">
                      <Checkbox
                        id={`all-${t.type}`}
                        checked={all}
                        disabled={disabled || full}
                        onCheckedChange={(c) => setType(t.type, c === true)}
                        aria-label={`All ${t.type} actions`}
                      />
                      <Label htmlFor={`all-${t.type}`} className="font-mono text-xs">{t.type}</Label>
                    </div>
                    <p className="mt-1 text-xs font-normal text-muted-foreground">{t.description}</p>
                  </th>
                  <td className="p-2">
                    <div className="flex flex-wrap gap-x-4 gap-y-2">
                      {t.actions.map((a) => {
                        const perm = `${t.type}:${a}`;
                        const id = `p-${t.type}-${a}`;
                        return (
                          <div key={a} className="flex items-center gap-1.5">
                            <Checkbox id={id} checked={all || set.has(perm)} disabled={disabled || all}
                              onCheckedChange={(c) => toggle(perm, c === true)} />
                            <Label htmlFor={id} className="text-xs">
                              {a}
                              {t.sensitive_actions.includes(a) && <span className="ml-1 text-amber-600" title="Sensitive: every use is logged">●</span>}
                            </Label>
                          </div>
                        );
                      })}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {exotic.length > 0 && (
        <p className="text-xs text-muted-foreground">Also includes: <span className="font-mono">{exotic.join(", ")}</span></p>
      )}
    </div>
  );
}
