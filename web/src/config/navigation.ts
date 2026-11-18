import { Activity, Building2, MonitorSmartphone, ScrollText, type LucideIcon } from "lucide-react";

export type NavItem = {
  title: string;
  href: string;
  icon: LucideIcon;
  /** Hide for non-owners (interim until S3 RBAC returns real permissions). */
  ownerOnly?: boolean;
};
export type NavSection = { title: string; items: NavItem[] };

/**
 * Navigation follows PRD §11. An entry is added in the same change that ships
 * its page, so the UI never links to unbuilt features. Visibility is UX only —
 * the API authorizes every request.
 */
export const navigation: NavSection[] = [
  {
    title: "Platform",
    items: [{ title: "Overview", href: "/", icon: Activity }],
  },
  {
    title: "Governance",
    items: [{ title: "Audit log", href: "/governance/audit", icon: ScrollText, ownerOnly: true }],
  },
  {
    title: "Settings",
    items: [
      { title: "Organization", href: "/settings/organization", icon: Building2 },
      { title: "My sessions", href: "/settings/sessions", icon: MonitorSmartphone },
    ],
  },
];
