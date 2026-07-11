-- name: ListDeploymentsForPullRequest :many
SELECT d.*
FROM deployments AS d
JOIN environments AS e ON e.id = d.environment_id
JOIN github_repo_connections AS c ON c.app_id = d.app_id
WHERE c.installation_id = sqlc.arg('installation_id')
  AND c.repository_id = sqlc.arg('repository_id')
  AND e.slug = 'preview'
  AND IF(
    CAST(sqlc.arg('is_fork_pr') AS SIGNED) = 1,
    d.pr_number = sqlc.arg('pr_number')
      AND d.fork_repository_full_name = sqlc.arg('fork_repository_full_name'),
    d.pr_number IS NULL
      AND d.git_branch = sqlc.arg('branch')
  )
ORDER BY d.created_at DESC;
