"use client";

import { PageBody, PageContainer, PageHeader, PageHeaderContent, PageHeaderTitle, SettingCardGroup } from "@unkey/ui";
import { Outputs } from "../settings/components/runtime-settings/outputs";
import { EnvironmentSettingsProvider } from "../settings/environment-provider";

export default function ServicesPage() {
  return (
    <EnvironmentSettingsProvider>
      <PageContainer>
        <PageHeader>
          <PageHeaderContent>
            <PageHeaderTitle>Services & Resources</PageHeaderTitle>
          </PageHeaderContent>
        </PageHeader>
        <PageBody>
          <div className="max-w-5xl">
            <SettingCardGroup>
              <Outputs />
            </SettingCardGroup>
          </div>
        </PageBody>
      </PageContainer>
    </EnvironmentSettingsProvider>
  );
}
