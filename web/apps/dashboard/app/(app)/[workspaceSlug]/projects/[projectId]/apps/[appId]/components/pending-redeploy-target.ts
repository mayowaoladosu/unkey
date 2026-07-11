type RedeployEnvironment = {
  id: string;
  slug: string;
};

type RedeployDeployment = {
  id: string;
  environmentId: string;
};

export function resolvePendingRedeployTarget<
  TEnvironment extends RedeployEnvironment,
  TDeployment extends RedeployDeployment,
>(
  changedEnvironmentIds: readonly string[],
  environments: readonly TEnvironment[],
  deployments: readonly TDeployment[],
): { environment: TEnvironment; deployment: TDeployment | undefined } | null {
  const changed = new Set(changedEnvironmentIds);
  const candidates = environments.filter((environment) => changed.has(environment.id));
  const environment =
    candidates.find((candidate) => candidate.slug === "production") ?? candidates.at(0);

  if (!environment) {
    return null;
  }

  return {
    environment,
    deployment: deployments.find(
      (deployment) => deployment.environmentId === environment.id,
    ),
  };
}
