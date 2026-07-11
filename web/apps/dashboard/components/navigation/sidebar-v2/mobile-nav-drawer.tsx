"use client";

import { useSidebar } from "@/components/ui/sidebar";
import { Logomark } from "@/components/logomark";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { routes } from "@/lib/navigation/routes";
import { Menu } from "@unkey/icons";
import { Drawer } from "@unkey/ui";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect } from "react";
import { SidebarBody } from "./sidebar-body";
import { HelpButton } from "../top-nav/help-button";
import { UserButton } from "../top-nav/user-button";
import { WorkspaceCrumb } from "../top-nav/workspace-crumb";

export function MobileNavDrawer() {
	const { isMobile, openMobile, setOpenMobile } = useSidebar();
	const pathname = usePathname();
	const workspace = useWorkspaceNavigation();

	// biome-ignore lint/correctness/useExhaustiveDependencies: pathname is the trigger
	useEffect(() => {
		setOpenMobile(false);
	}, [pathname, setOpenMobile]);

	if (!isMobile) {
		return null;
	}

	return (
		<Drawer.Root open={openMobile} onOpenChange={setOpenMobile}>
			<Drawer.Content>
				<Drawer.Title className="sr-only">Navigation</Drawer.Title>
				<Drawer.Description className="sr-only">
					Navigate to sections and sub-pages of the dashboard.
				</Drawer.Description>
				<div className="flex items-center gap-1 border-b border-grayA-4 p-3">
					<Link
						href={routes.workspaces.overview({ workspaceSlug: workspace.slug })}
						aria-label="LayerRail dashboard"
						className="inline-flex size-8 items-center justify-center"
					>
						<Logomark />
					</Link>
					<WorkspaceCrumb href={routes.workspaces.overview({ workspaceSlug: workspace.slug })} />
				</div>
				<div className="max-h-[70svh] overflow-y-auto">
					<SidebarBody />
				</div>
				<div className="flex items-center gap-2 border-t border-grayA-4 p-3">
					<HelpButton />
					<UserButton className="flex-1" />
				</div>
			</Drawer.Content>
		</Drawer.Root>
	);
}

export function MobileNavTrigger() {
	const { isMobile, setOpenMobile } = useSidebar();
	if (!isMobile) {
		return null;
	}
	return (
		<button
			type="button"
			onClick={() => setOpenMobile(true)}
			aria-label="Open navigation"
			className="fixed bottom-4 left-4 z-40 flex h-10 items-center gap-2 rounded-full border border-grayA-5 bg-gray-1 px-3 text-xs font-medium text-gray-12 shadow-lg"
		>
			<Menu iconSize="sm-regular" />
			Navigation
		</button>
	);
}
