"use client";

import {
  PageBody,
  PageContainer,
  PageHeader,
  PageHeaderContent,
  PageHeaderTitle,
  SettingsDangerZone,
} from "@unkey/ui";
import { DeleteProject } from "./components/delete-project";
import { SlackIntegration } from "./components/slack-integration";
import { UpdateProjectSettings } from "./components/update-project-settings";

export default function ProjectSettingsPage() {
  return (
    <PageContainer>
      <PageHeader>
        <PageHeaderContent>
          <PageHeaderTitle>Project Settings</PageHeaderTitle>
        </PageHeaderContent>
      </PageHeader>
      <PageBody>
        <UpdateProjectSettings />
        <SlackIntegration />
        <SettingsDangerZone>
          <DeleteProject />
        </SettingsDangerZone>
      </PageBody>
    </PageContainer>
  );
}
