package environment

import (
	restateingress "github.com/restatedev/sdk-go/ingress"
	"github.com/unkeyed/unkey/gen/proto/ctrl/v1/ctrlv1connect"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auditlogs"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

// Service owns the complete control-plane lifecycle for app environments.
// Creation and metadata changes commit atomically; destructive cleanup is
// delegated to the existing durable Restate virtual object.
type Service struct {
	ctrlv1connect.UnimplementedEnvironmentServiceHandler
	db        db.Database
	restate   *restateingress.Client
	auditlogs auditlogs.AuditLogService
	bearer    string
}

type Config struct {
	Database  db.Database
	Restate   *restateingress.Client
	Auditlogs auditlogs.AuditLogService
	Bearer    string
}

func New(cfg Config) *Service {
	return &Service{
		UnimplementedEnvironmentServiceHandler: ctrlv1connect.UnimplementedEnvironmentServiceHandler{},
		db:                                     cfg.Database,
		restate:                                cfg.Restate,
		auditlogs:                              cfg.Auditlogs,
		bearer:                                 cfg.Bearer,
	}
}