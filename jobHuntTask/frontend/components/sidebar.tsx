"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import {
  Briefcase,
  Calendar,
  GraduationCap,
  LayoutDashboard,
  ListChecks,
  MessageSquare,
  Mic,
  Sparkles,
  TrendingUp,
  User,
} from "lucide-react";

const links = [
  { href: "/", label: "Today", icon: LayoutDashboard },
  { href: "/profile", label: "Profile", icon: User },
  { href: "/jobs", label: "Fit Matches", icon: Briefcase },
  { href: "/applications", label: "Pipeline", icon: Calendar },
  { href: "/outreach", label: "Outreach", icon: MessageSquare },
  { href: "/market", label: "Market", icon: TrendingUp },
  { href: "/skills", label: "Skills", icon: GraduationCap },
  { href: "/interview", label: "Interview", icon: Mic },
  { href: "/coach", label: "Coach", icon: Sparkles },
];

export function Sidebar() {
  const pathname = usePathname();
  return (
    <aside className="flex w-56 shrink-0 flex-col border-r border-border bg-card/50 p-4">
      <div className="mb-8 px-2">
        <p className="text-xs font-medium uppercase tracking-widest text-muted">
          Career OS
        </p>
        <h1 className="text-lg font-semibold">Your recruiter</h1>
      </div>
      <nav className="flex flex-1 flex-col gap-1">
        <a
          href="/dashboard"
          className="mb-2 flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted transition-colors hover:bg-white/5 hover:text-foreground"
        >
          <ListChecks className="h-4 w-4" />
          Task tracker
        </a>
        {links.map(({ href, label, icon: Icon }) => {
          const active =
            pathname === href ||
            pathname === href.replace(/\/$/, "") ||
            pathname + "/" === href;
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors",
                active
                  ? "bg-white/10 text-foreground"
                  : "text-muted hover:bg-white/5 hover:text-foreground"
              )}
            >
              <Icon className="h-4 w-4" />
              {label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
