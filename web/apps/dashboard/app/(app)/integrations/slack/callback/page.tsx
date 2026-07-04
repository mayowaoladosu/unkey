"use client";
import { LoadingState } from "@/components/loading-state";
import { trpc } from "@/lib/trpc/client";
import { Empty, toast } from "@unkey/ui";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect } from "react";

export default function Page() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const state = searchParams?.get("state") ?? null;
  const code = searchParams?.get("code") ?? null;
  // Slack sets `error` (e.g. "access_denied") when the user declines consent.
  const error = searchParams?.get("error") ?? null;

  const mutation = trpc.slack.registerInstallation.useMutation({
    onSuccess: (data) => {
      toast.success(`Connected Slack workspace ${data.teamName}`);
      router.replace("/");
    },
    onError: (err) => {
      toast.error(err.message);
    },
  });

  useEffect(() => {
    if (error || !state || !code) {
      return;
    }
    if (mutation.isIdle) {
      mutation.mutate({ state, code });
    }
  }, [mutation, state, code, error]);

  if (error) {
    return (
      <div className="w-full min-h-[60vh] flex justify-center items-center">
        <Empty>
          <Empty.Title>Slack authorization cancelled</Empty.Title>
          <Empty.Description>Slack returned: {error}</Empty.Description>
        </Empty>
      </div>
    );
  }

  if (!state || !code) {
    return (
      <div className="w-full min-h-[60vh] flex justify-center items-center">
        <Empty>
          <Empty.Title>Invalid callback</Empty.Title>
          <Empty.Description>Missing Slack authorization state or code.</Empty.Description>
        </Empty>
      </div>
    );
  }

  if (mutation.isError) {
    return (
      <div className="w-full min-h-[60vh] flex justify-center items-center">
        <Empty>
          <Empty.Title>Slack connection failed</Empty.Title>
          <Empty.Description>{mutation.error.message}</Empty.Description>
        </Empty>
      </div>
    );
  }

  return <LoadingState message="Finalizing Slack connection..." />;
}
