"use client";

import { Combobox, type ComboboxOption } from "@/components/ui/combobox";
import { Switch } from "@/components/ui/switch";
import { trpc } from "@/lib/trpc/client";
import { Trash } from "@unkey/icons";
import {
  Button,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  SettingCard,
  SettingCardGroup,
  toast,
} from "@unkey/ui";
import { useParams } from "next/navigation";

// SlackIntegration renders the per-project Slack notification settings: connect
// the workspace via OAuth, fan notifications out to any number of channels
// (each with its own production/preview scope), and set the project-level
// approval policy. All mutations are admin-gated server-side.
export const SlackIntegration = () => {
  const { projectId } = useParams<{ projectId: string }>();
  const utils = trpc.useUtils();

  const installation = trpc.slack.hasInstallation.useQuery();
  const installed = installation.data?.installed ?? false;

  const connections = trpc.slack.listConnections.useQuery({ projectId }, { enabled: installed });
  const slackChannels = trpc.slack.listChannels.useQuery(undefined, {
    enabled: installed,
    retry: false,
  });

  const connected = connections.data?.channels ?? [];
  const approvalPolicy = connections.data?.approvalPolicy ?? "anyone";

  const invalidate = () => utils.slack.listConnections.invalidate({ projectId });

  const prepare = trpc.slack.prepareInstallation.useMutation({
    onError: (err) => toast.error(err.message),
  });
  const addChannel = trpc.slack.addChannel.useMutation({
    onSuccess: invalidate,
    onError: (err) => toast.error(err.message),
  });
  const removeChannel = trpc.slack.removeChannel.useMutation({
    onSuccess: invalidate,
    onError: (err) => toast.error(err.message),
  });
  const updateScope = trpc.slack.updateChannelScope.useMutation({
    onSuccess: invalidate,
    onError: (err) => toast.error(err.message),
  });
  const setPolicy = trpc.slack.setApprovalPolicy.useMutation({
    onSuccess: invalidate,
    onError: (err) => toast.error(err.message),
  });
  const sendTest = trpc.slack.sendTestMessage.useMutation({
    onSuccess: () => toast.success("Test message sent"),
    onError: (err) => toast.error(err.message),
  });

  const handleConnect = async () => {
    try {
      const { url } = await prepare.mutateAsync();
      window.location.href = url;
    } catch {
      // onError toast already shown.
    }
  };

  if (!installed) {
    return (
      <SettingCardGroup>
        <SettingCard
          title="Slack notifications"
          description="Connect Slack to receive deployment notifications and approve gated deployments from your channels."
          contentWidth="w-auto"
        >
          <div className="flex justify-end">
            <Button variant="primary" onClick={handleConnect} loading={prepare.isLoading}>
              Add to Slack
            </Button>
          </div>
        </SettingCard>
      </SettingCardGroup>
    );
  }

  // Channels not yet connected to this project, for the picker.
  const connectedIds = new Set(connected.map((c) => c.channelId));
  const options: ComboboxOption[] = (slackChannels.data?.channels ?? [])
    .filter((c) => !connectedIds.has(c.id))
    .map((c) => ({
      value: c.id,
      label: `#${c.name}`,
      searchValue: c.name,
    }));

  return (
    <SettingCardGroup>
      <SettingCard
        title="Notification channels"
        description="Deployment notifications are sent to every connected channel. Toggle which environments each channel receives."
        contentWidth="w-full max-w-[560px]"
      >
        <div className="flex w-full flex-col gap-3">
          <Combobox
            options={options}
            value=""
            onSelect={(channelId) => {
              const channel = slackChannels.data?.channels.find((c) => c.id === channelId);
              if (channel) {
                addChannel.mutate({ projectId, channelId: channel.id, channelName: channel.name });
              }
            }}
            placeholder={<span className="text-gray-9">Add a channel...</span>}
            searchPlaceholder="Search channels..."
            emptyMessage={
              <div className="mt-2 text-gray-9 text-[13px]">
                {slackChannels.isError ? "Could not load channels" : "No channels found"}
              </div>
            }
            disabled={addChannel.isLoading}
          />

          {connected.length === 0 ? (
            <span className="text-[13px] text-gray-9 py-1">
              No channels connected yet. Add one to start receiving deployment notifications.
            </span>
          ) : (
            <div className="flex flex-col divide-y divide-grayA-3">
              {connected.map((channel) => (
                <div key={channel.channelId} className="flex items-center gap-4 py-2.5">
                  <span className="flex-1 truncate text-[13px] font-medium text-gray-12">
                    #{channel.channelName}
                  </span>
                  <label className="flex items-center gap-2">
                    <Switch
                      size="sm"
                      checked={channel.notifyProduction}
                      onCheckedChange={(checked) =>
                        updateScope.mutate({
                          projectId,
                          channelId: channel.channelId,
                          notifyProduction: checked,
                          notifyPreviews: channel.notifyPreviews,
                        })
                      }
                    />
                    <span className="text-[13px] text-gray-11">Production</span>
                  </label>
                  <label className="flex items-center gap-2">
                    <Switch
                      size="sm"
                      checked={channel.notifyPreviews}
                      onCheckedChange={(checked) =>
                        updateScope.mutate({
                          projectId,
                          channelId: channel.channelId,
                          notifyProduction: channel.notifyProduction,
                          notifyPreviews: checked,
                        })
                      }
                    />
                    <span className="text-[13px] text-gray-11">Preview</span>
                  </label>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => sendTest.mutate({ projectId, channelId: channel.channelId })}
                    loading={
                      sendTest.isLoading && sendTest.variables?.channelId === channel.channelId
                    }
                  >
                    Send test
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="size-8 shrink-0 px-0 text-gray-9 hover:text-error-11"
                    aria-label={`Remove #${channel.channelName}`}
                    onClick={() =>
                      removeChannel.mutate({ projectId, channelId: channel.channelId })
                    }
                  >
                    <Trash iconSize="sm-regular" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </div>
      </SettingCard>

      <SettingCard
        title="Approval policy"
        description="Who can approve or reject gated deployments from Slack."
        contentWidth="w-full max-w-[560px]"
      >
        <Select
          value={approvalPolicy}
          onValueChange={(value) =>
            setPolicy.mutate({ projectId, approvalPolicy: value as "anyone" | "admins_only" })
          }
          disabled={connected.length === 0 || setPolicy.isLoading}
        >
          <SelectTrigger>
            <SelectValue placeholder="Select policy" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="anyone">Anyone in the channel</SelectItem>
            <SelectItem value="admins_only">Workspace admins only</SelectItem>
          </SelectContent>
        </Select>
      </SettingCard>
    </SettingCardGroup>
  );
};
