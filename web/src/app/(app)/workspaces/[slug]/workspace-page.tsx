"use client";

import { useParams } from "next/navigation";
import { WorkspaceDetail } from "@/features/workspaces/components/workspace-detail";

export function WorkspacePage() {
  const { slug } = useParams<{ slug: string }>();
  return <WorkspaceDetail slug={slug} />;
}
