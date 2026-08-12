import {
  ChevronsUpDownIcon,
  GaugeIcon,
  KeyRoundIcon,
  LogOutIcon,
  RegexIcon,
  RouteIcon,
  ScrollTextIcon,
  ServerIcon,
  SettingsIcon,
  ShieldIcon,
  WaypointsIcon,
  ZapIcon,
} from "lucide-react";
import { NavLink, useLocation } from "react-router";

import { useAuth } from "@/components/auth-provider";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { useT, type TranslationKey } from "@/lib/i18n";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";

// Surfaced next to the wordmark so a screenshot in a bug report always
// says which build it came from.
const APP_VERSION = "v0.1";

const GROUPS: {
  label: TranslationKey;
  items: { title: TranslationKey; url: string; icon: typeof GaugeIcon }[];
}[] = [
  {
    label: "nav.group.gateway",
    items: [
      { title: "nav.overview", url: "/", icon: GaugeIcon },
      { title: "nav.providers", url: "/providers", icon: ServerIcon },
      { title: "nav.routes", url: "/routes", icon: RouteIcon },
    ],
  },
  {
    label: "nav.group.resolution",
    items: [
      { title: "nav.virtualModels", url: "/virtual-models", icon: WaypointsIcon },
      { title: "nav.regexModels", url: "/regex-models", icon: RegexIcon },
    ],
  },
  {
    label: "nav.group.access",
    items: [
      { title: "nav.keys", url: "/keys", icon: KeyRoundIcon },
      { title: "nav.logs", url: "/logs", icon: ScrollTextIcon },
      { title: "nav.dlp", url: "/dlp", icon: ShieldIcon },
    ],
  },
];

export function AppSidebar() {
  const t = useT();
  const { user, logout } = useAuth();
  const { pathname } = useLocation();

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <NavLink to="/">
                <div className="bg-sidebar-primary text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg">
                  <ZapIcon className="size-4" />
                </div>
                <div className="grid flex-1 text-left leading-tight">
                  <span className="flex items-center gap-1.5">
                    <span className="truncate text-sm font-semibold tracking-tight">
                      {t("app.name")}
                    </span>
                    <span className="bg-muted text-muted-foreground rounded px-1 py-px font-mono text-[10px]">
                      {APP_VERSION}
                    </span>
                  </span>
                  <span className="text-muted-foreground truncate text-xs">{t("app.subtitle")}</span>
                </div>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        {GROUPS.map((group) => (
          <SidebarGroup key={group.label}>
            <SidebarGroupLabel>{t(group.label)}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((item) => (
                  <SidebarMenuItem key={item.url}>
                    <SidebarMenuButton
                      asChild
                      isActive={pathname === item.url}
                      tooltip={t(item.title)}
                    >
                      <NavLink to={item.url}>
                        <item.icon />
                        <span>{t(item.title)}</span>
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton size="lg">
                  <Avatar className="size-8 rounded-lg">
                    <AvatarFallback className="rounded-lg">
                      {(user?.nickname || user?.username || "?").slice(0, 2).toUpperCase()}
                    </AvatarFallback>
                  </Avatar>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">
                      {user?.nickname || user?.username}
                    </span>
                    <span className="text-muted-foreground truncate text-xs">
                      {user?.email || t("nav.noEmail")}
                    </span>
                  </div>
                  <ChevronsUpDownIcon className="ml-auto size-4" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" className="w-56">
                <DropdownMenuLabel>{t("nav.signedInAs", { username: user?.username ?? "" })}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem asChild>
                  <NavLink to="/settings">
                    <SettingsIcon />
                    {t("nav.settings")}
                  </NavLink>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => void logout()}>
                  <LogOutIcon />
                  {t("nav.signOut")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
