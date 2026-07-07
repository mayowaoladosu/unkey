package db

// GetID exposes the primary id as a method so cursor-paginated list handlers can
// treat rows through a shared Identifiable constraint. Hand-written because sqlc
// emits id as a field, not a method; kept out of the generated files so it
// survives regeneration.
//
// To add a new paginated list endpoint, add a GetID method for its row type here
// so it satisfies the Identifiable constraint used by PaginateByID in
// [svc/api/internal/pagination/pagination.go].

func (r App) GetID() string { return r.ID }

func (r ListProjectsByWorkspaceIdRow) GetID() string { return r.ID }

func (r Permission) GetID() string { return r.ID }

func (r ListRolesRow) GetID() string { return r.ID }

func (r ListIdentitiesRow) GetID() string { return r.ID }

func (r ListAppEnvVarsByAppAndEnvRow) GetID() string { return r.ID }

func (r ListLiveKeysByKeySpaceIDRow) GetID() string { return r.ID }

func (r RatelimitOverride) GetID() string { return r.ID }
