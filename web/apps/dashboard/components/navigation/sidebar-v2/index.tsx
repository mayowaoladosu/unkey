"use client";

import {
	Sidebar,
	SidebarContent,
	SidebarFooter,
	SidebarHeader,
	useSidebar,
} from "@/components/ui/sidebar";
import { Logomark } from "@/components/logomark";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { routes } from "@/lib/navigation/routes";
import { cn } from "@/lib/utils";
import { SidebarLeftHide, SidebarLeftShow } from "@unkey/icons";
import { Tooltip, TooltipContent, TooltipTrigger } from "@unkey/ui";
import Link from "next/link";
import { TopNavFeedbackButton } from "../top-nav/feedback-button";
import { HelpButton } from "../top-nav/help-button";
import { UserButton } from "../top-nav/user-button";
import { WorkspaceCrumb } from "../top-nav/workspace-crumb";
import { SidebarBody } from "./sidebar-body";
import { UsageBanner } from "./usage-banner";

export const SIDEBAR_WIDTH_VARS: React.CSSProperties & {
	"--sidebar-width": string;
	"--sidebar-width-icon": string;
} = {
	"--sidebar-width": "13rem",
	"--sidebar-width-icon": "3rem",
};

type Props = React.ComponentProps<typeof Sidebar>;

export function SidebarV2(props: Props) {
	const { isMobile, state } = useSidebar();
	const workspace = useWorkspaceNavigation();
	const collapsed = state === "collapsed";
	if (isMobile) {
		return null;
	}
	return (
		<Sidebar
			{...props}
			collapsible="icon"
			className={cn("[&_[data-sidebar=sidebar]]:bg-gray-1", props.className)}
			style={{ top: 0, height: "100svh" }}
		>
			<SidebarHeader className="border-b border-grayA-4 p-2">
				<div className={cn("flex h-9 items-center", collapsed ? "justify-center" : "gap-1")}>
					<Link
						href={routes.workspaces.overview({ workspaceSlug: workspace.slug })}
						aria-label="LayerRail dashboard"
						className="inline-flex size-8 shrink-0 items-center justify-center rounded-md hover:bg-grayA-3"
					>
						<Logomark />
					</Link>
					{!collapsed ? (
						<div className="min-w-0 flex-1 overflow-hidden">
							<WorkspaceCrumb
								href={routes.workspaces.overview({ workspaceSlug: workspace.slug })}
							/>
						</div>
					) : null}
				</div>
			</SidebarHeader>
			<SidebarContent>
				<SidebarBody />
			</SidebarContent>
			<SidebarFooter className="mx-0 gap-2 border-t-0 p-2">
				<UsageBanner />
				{!collapsed ? <TopNavFeedbackButton className="w-full justify-start" /> : null}
				<div className={cn("flex items-center gap-1", !collapsed && "justify-between")}>
					<HelpButton />
					<UserButton isCollapsed={collapsed} className={collapsed ? undefined : "flex-1"} />
				</div>
				<CollapseButton />
			</SidebarFooter>
		</Sidebar>
	);
}

function CollapseButton() {
	const { state, toggleSidebar } = useSidebar();
	const collapsed = state === "collapsed";
	const Icon = collapsed ? SidebarLeftShow : SidebarLeftHide;
	const label = collapsed ? "Expand sidebar" : "Collapse sidebar";
	return (
		<Tooltip>
			<TooltipTrigger asChild>
				<button
					type="button"
					onClick={toggleSidebar}
					aria-label={label}
					className="flex size-8 items-center justify-center rounded-md text-gray-11 hover:bg-grayA-3 hover:text-gray-12"
				>
					<Icon iconSize="md-regular" className="shrink-0" />
				</button>
			</TooltipTrigger>
			<TooltipContent
				side="right"
				align="center"
				className="dark:bg-white bg-black text-gray-1 px-2 py-1 border border-accent-6 shadow-md font-medium text-xs"
			>
				{label}
			</TooltipContent>
		</Tooltip>
	);
}
