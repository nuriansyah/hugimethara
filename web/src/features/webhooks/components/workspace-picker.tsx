"use client";

import { Label } from "@hugimethara/ui/components/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@hugimethara/ui/components/select";
import { useWorkspaces } from "@/features/workspaces/api";

/** Workspace selector; defaults to the first workspace. Returns the resolved id. */
export function useWorkspaceChoice(chosen: string) {
  const workspaces = useWorkspaces(false);
  return { workspaces, resolved: chosen || workspaces.data?.[0]?.id || "" };
}

export function WorkspacePicker({
  id,
  value,
  onChange,
  workspaces,
}: {
  id: string;
  value: string;
  onChange: (v: string) => void;
  workspaces: { id: string; name: string }[];
}) {
  return (
    <div className="flex min-w-48 max-w-xs flex-col gap-1.5">
      <Label htmlFor={id}>Workspace</Label>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id} className="w-full">
          <SelectValue placeholder="Choose a workspace" />
        </SelectTrigger>
        <SelectContent>
          {workspaces.map((w) => (
            <SelectItem key={w.id} value={w.id}>
              {w.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
