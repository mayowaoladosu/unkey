"use client";

import { trpc } from "@/lib/trpc/client";
import { Button, SettingCard, toast } from "@unkey/ui";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

// SlackIntegration renders the per-project Slack notification settings:
// connect (workspace OAuth install), pick a channel, choose environment scope
// and approval policy, send a test message, and disconnect. All mutations are
// admin-gated server-side; non-admins see the server's FORBIDDEN error.
export const SlackIntegration = () => {
  const { projectId } = useParams<{ projectId: string }>();
  const utils = trpc.useUtils();

  const installation = trpc.slack.hasInstallation.useQuery();
  const connection = trpc.slack.getConnection.useQuery({ projectId });
  const installed = installation.data?.installed ?? false;

  const channels = trpc.slack.listChannels.useQuery(undefined, { enabled: installed });

  const [channelId, setChannelId] = useState("");
  const [includePreviews, setIncludePreviews] = useState(false);
  const [approvalPolicy, setApprovalPolicy] = useState<"anyone" | "admins_only">("anyone");

  // Sync local form state from the persisted connection when it loads.
  useEffect(() => {
    if (connection.data) {
      setChannelId(connection.data.channelId);
      setIncludePreviews(connection.data.includePreviews);
      setApprovalPolicy(connection.data.approvalPolicy);
    }
  }, [connection.data]);

  const prepare = trpc.slack.prepareInstallation.useMutation({
    onError: (err) => toast.error(err.message),
  });
  const selectChannel = trpc.slack.selectChannel.useMutation({
    onSuccess: () => {
      toast.success("Channel connected");
      utils.slack.getConnection.invalidate({ projectId });
    },
    onError: (err) => toast.error(err.message),
  });
  const updateConfig = trpc.slack.updateConfig.useMutation({
    onSuccess: () => {
      toast.success("Saved");
      utils.slack.getConnection.invalidate({ projectId });
    },
    onError: (err) => toast.error(err.message),
  });
  const sendTest = trpc.slack.sendTestMessage.useMutation({
    onSuccess: () => toast.success("Test message sent"),
    onError: (err) => toast.error(err.message),
  });
  const disconnect = trpc.slack.disconnect.useMutation({
    onSuccess: () => {
      toast.success("Disconnected");
      utils.slack.getConnection.invalidate({ projectId });
    },
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
      <SettingCard
        title="Slack notifications"
        description="Connect Slack to receive deployment notifications and approve gated deployments from a channel."
        border="both"
      >
        <div className="flex justify-end w-full">
          <Button variant="primary" onClick={handleConnect} loading={prepare.isLoading}>
            Add to Slack
          </Button>
        </div>
      </SettingCard>
    );
  }

  const selectedChannel = channels.data?.channels.find((c) => c.id === channelId);

  return (
    <SettingCard
      title="Slack notifications"
      description="Choose a channel, environment scope, and who can approve gated deployments from Slack."
      border="both"
    >
      <div className="flex flex-col gap-4 w-full">
        <label className="flex flex-col gap-1 text-[13px] text-gray-11">
          Channel
          <select
            className="rounded-lg border border-grayA-5 bg-transparent px-3 py-2 text-gray-12"
            value={channelId}
            onChange={(e) => setChannelId(e.target.value)}
          >
            <option value="">Select a channel…</option>
            {channels.data?.channels.map((c) => (
              <option key={c.id} value={c.id}>
                #{c.name}
              </option>
            ))}
          </select>
        </label>

        <label className="flex items-center gap-2 text-[13px] text-gray-11">
          <input
            type="checkbox"
            checked={includePreviews}
            onChange={(e) => setIncludePreviews(e.target.checked)}
          />
          Include preview deployments (production always notifies)
        </label>

        <label className="flex flex-col gap-1 text-[13px] text-gray-11">
          Who can approve gated deployments
          <select
            className="rounded-lg border border-grayA-5 bg-transparent px-3 py-2 text-gray-12"
            value={approvalPolicy}
            onChange={(e) => setApprovalPolicy(e.target.value as "anyone" | "admins_only")}
          >
            <option value="anyone">Anyone in the channel</option>
            <option value="admins_only">Workspace admins only</option>
          </select>
        </label>

        <div className="flex flex-wrap justify-end gap-2">
          <Button
            variant="outline"
            onClick={() => disconnect.mutate({ projectId })}
            loading={disconnect.isLoading}
          >
            Disconnect
          </Button>
          <Button
            variant="outline"
            onClick={() => sendTest.mutate({ projectId })}
            loading={sendTest.isLoading}
            disabled={!connection.data}
          >
            Send test
          </Button>
          <Button
            variant="outline"
            onClick={() => updateConfig.mutate({ projectId, includePreviews, approvalPolicy })}
            loading={updateConfig.isLoading}
            disabled={!connection.data}
          >
            Save settings
          </Button>
          <Button
            variant="primary"
            onClick={() => {
              if (selectedChannel) {
                selectChannel.mutate({
                  projectId,
                  channelId: selectedChannel.id,
                  channelName: selectedChannel.name,
                });
              }
            }}
            loading={selectChannel.isLoading}
            disabled={!selectedChannel}
          >
            {connection.data ? "Update channel" : "Connect channel"}
          </Button>
        </div>
      </div>
    </SettingCard>
  );
};
