-- name: ListStripeConnectedWorkspaces :many
SELECT *
FROM workspace_billing_settings
WHERE stripe_connect_encrypted IS NOT NULL
ORDER BY pk
LIMIT ? OFFSET ?;
