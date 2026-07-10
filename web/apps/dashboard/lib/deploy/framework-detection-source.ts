type GitSourceIdentity = {
  repositoryFullName: string;
  branch: string;
};

export function detectionMatchesGitSource(
  detectionSource: GitSourceIdentity,
  currentSource: GitSourceIdentity,
): boolean {
  return (
    detectionSource.repositoryFullName === currentSource.repositoryFullName &&
    detectionSource.branch === currentSource.branch
  );
}
