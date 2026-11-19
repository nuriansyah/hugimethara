"use client";

import { useParams } from "next/navigation";
import { InviteAccept } from "@/features/identity/components/invite-accept";

export function InvitePage() {
  const { token } = useParams<{ token: string }>();
  return <InviteAccept token={token} />;
}
