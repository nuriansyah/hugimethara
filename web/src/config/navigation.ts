import {
  Activity,
  Bot,
  Building2,
  CreditCard,
  Fingerprint,
  FolderKanban,
  Key,
  KeyRound,
  Layers,
  MonitorSmartphone,
  Network,
  PlugZap,
  Code2,
  ScrollText,
  ShieldCheck,
  UserRound,
  UsersRound,
  type LucideIcon,
} from "lucide-react";

export type NavItem = {
  title: string;
  href: string;
  icon: LucideIcon;
  /** Show only to callers holding this organization-level permission (UX only). */
  permission?: string;
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
    title: "Workspace",
    items: [{ title: "Workspaces", href: "/workspaces", icon: FolderKanban }],
  },
  {
    title: "Integrations",
    items: [
      { title: "Connectors", href: "/integrations/connectors", icon: PlugZap, permission: "connector:read" },
      { title: "Service registry", href: "/integrations/services", icon: Code2, permission: "service:read" },
    ],
  },
  {
    title: "People",
    items: [
      { title: "Members", href: "/settings/members", icon: UserRound, permission: "user:read" },
      { title: "Groups", href: "/settings/groups", icon: UsersRound, permission: "group:read" },
      { title: "Service accounts", href: "/settings/service-accounts", icon: Bot, permission: "service_account:read" },
    ],
  },
  {
    title: "Governance",
    items: [
      { title: "Roles", href: "/governance/roles", icon: ShieldCheck, permission: "role:read" },
      { title: "Access", href: "/governance/access", icon: KeyRound, permission: "role:read" },
      { title: "Audit log", href: "/governance/audit", icon: ScrollText, permission: "audit_event:read" },
    ],
  },
  {
    title: "Settings",
    items: [
      { title: "Single sign-on", href: "/settings/identity-providers", icon: Fingerprint, permission: "identity_provider:read" },
      { title: "Credentials", href: "/settings/credentials", icon: Key, permission: "secret:read" },
      { title: "Organization", href: "/settings/organization", icon: Building2 },
      { title: "Departments", href: "/settings/departments", icon: Layers },
      { title: "Environments", href: "/settings/environments", icon: Network },
      { title: "Plan & usage", href: "/settings/plan", icon: CreditCard },
      { title: "My sessions", href: "/settings/sessions", icon: MonitorSmartphone },
    ],
  },
];
