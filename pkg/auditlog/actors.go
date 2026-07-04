package auditlog

// AuditLogActor represents the type of entity that performed an action.
// Actors are categorized to enable filtering and analysis of audit logs
// based on the source of actions.
type AuditLogActor string

const (
	// RootKeyActor indicates the action was performed using a root API key.
	// Root keys can manage workspace resources.
	RootKeyActor AuditLogActor = "rootkey"

	// UserActor indicates the action was performed by a human user
	// directly interacting with the system, typically through the UI.
	UserActor AuditLogActor = "user"

	// SystemActor indicates the action was performed automatically by
	// the system itself, without direct human intervention.
	// This might include scheduled tasks, automatic cleanups, or
	// system maintenance operations.
	SystemActor AuditLogActor = "system"

	// PortalEndUserActor indicates the action was performed by an end user
	// authenticated through a customer portal session, rather than by the
	// workspace owner. The actor's externalId is carried by the audit log's
	// ActorID so customers can see what their end users did.
	PortalEndUserActor AuditLogActor = "portalEndUser"

	// SlackActor indicates the action was performed by a Slack user acting
	// through the Slack integration (e.g. approving or rejecting a gated
	// deployment). When the Slack user maps to an Unkey identity that id is
	// recorded; otherwise the Slack user id/name is carried in ActorID/meta.
	SlackActor AuditLogActor = "slack"
)
