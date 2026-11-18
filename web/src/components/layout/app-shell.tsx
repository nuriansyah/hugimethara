"use client";

import { LogOut } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import type { ReactNode } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import { navigation } from "@/config/navigation";
import { useLogout } from "@/features/auth/api";
import { useCurrentUser } from "@/features/auth/components/auth-gate";
import { cn } from "@/lib/utils";

export function AppShell({ children }: { children: ReactNode }) {
  const me = useCurrentUser();
  const pathname = usePathname();
  const router = useRouter();
  const logout = useLogout();

  async function signOut() {
    try {
      await logout.mutateAsync();
      router.replace("/login");
    } catch {
      toast.error("Sign-out failed. Try again.");
    }
  }

  const sections = navigation
    .map((s) => ({ ...s, items: s.items.filter((i) => !i.ownerOnly || me.membership.is_owner) }))
    .filter((s) => s.items.length > 0);

  return (
    <div className="flex min-h-screen flex-col md:flex-row">
      <aside className="flex flex-col border-b bg-sidebar text-sidebar-foreground md:w-60 md:shrink-0 md:border-r md:border-b-0">
        <div className="flex h-14 items-center gap-2 px-4">
          <span className="flex size-7 items-center justify-center rounded-md bg-primary text-xs font-bold text-primary-foreground">
            H
          </span>
          <span className="truncate font-semibold tracking-tight" title={me.organization.name}>
            {me.organization.name}
          </span>
        </div>
        <Separator />
        <nav aria-label="Main" className="flex flex-1 gap-4 overflow-x-auto p-3 md:flex-col">
          {sections.map((section) => (
            <div key={section.title} className="flex flex-col gap-1">
              <p className="px-2 text-xs font-medium text-muted-foreground uppercase">{section.title}</p>
              {section.items.map((item) => {
                const active = item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    aria-current={active ? "page" : undefined}
                    className={cn(
                      "flex items-center gap-2 rounded-md px-2 py-1.5 text-sm whitespace-nowrap hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
                      active && "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
                    )}
                  >
                    <item.icon className="size-4" aria-hidden />
                    {item.title}
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>
        <Separator />
        <div className="p-3">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="w-full justify-start gap-2 px-2">
                <span className="flex size-7 items-center justify-center rounded-full bg-muted text-xs font-semibold uppercase">
                  {me.user.display_name.slice(0, 1)}
                </span>
                <span className="flex min-w-0 flex-col items-start text-left">
                  <span className="truncate text-sm">{me.user.display_name}</span>
                  <span className="truncate text-xs text-muted-foreground">{me.user.email}</span>
                </span>
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-56">
              <DropdownMenuLabel className="font-normal">
                <span className="block text-sm">{me.user.display_name}</span>
                <span className="block text-xs text-muted-foreground">
                  {me.membership.is_owner ? "Owner" : "Member"} · {me.organization.slug}
                </span>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuItem asChild>
                <Link href="/settings/sessions">My sessions</Link>
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={signOut} disabled={logout.isPending}>
                <LogOut aria-hidden /> Sign out
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </aside>
      <main className="flex-1 p-4 md:p-8">{children}</main>
    </div>
  );
}
