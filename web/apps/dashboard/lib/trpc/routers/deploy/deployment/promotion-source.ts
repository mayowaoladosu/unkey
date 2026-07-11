type PromotionSource = {
  image: string | null;
  gitCommitSha: string | null;
  gitBranch: string | null;
  gitCommitMessage: string | null;
  gitCommitAuthorHandle: string | null;
  gitCommitAuthorAvatarUrl: string | null;
  gitCommitTimestamp: number | null;
  forkRepositoryFullName: string | null;
};

export function resolveProductionSource(source: PromotionSource, hasRepoConnection: boolean) {
  const hasGitSource = hasRepoConnection && Boolean(source.gitCommitSha || source.gitBranch);
  if (!source.image && !hasGitSource) {
    return null;
  }

  return {
    ...(source.image ? { dockerImage: source.image } : {}),
    ...(hasGitSource
      ? {
          gitCommit: {
            commitSha: source.gitCommitSha ?? "",
            branch: source.gitBranch ?? "",
            commitMessage: source.gitCommitMessage ?? "",
            authorHandle: source.gitCommitAuthorHandle ?? "",
            authorAvatarUrl: source.gitCommitAuthorAvatarUrl ?? "",
            timestamp: BigInt(source.gitCommitTimestamp ?? 0),
            forkRepository: source.forkRepositoryFullName ?? "",
          },
        }
      : {}),
  };
}
