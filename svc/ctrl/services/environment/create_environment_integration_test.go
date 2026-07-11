package environment_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/pkg/mysql/sqlcomment"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/ctrl/integration/seed"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auditlogs"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	environmentservice "github.com/unkeyed/unkey/svc/ctrl/services/environment"
)

func TestEnvironmentLifecycleClonesConfigurationAtomically(t *testing.T) {
	ctx := context.Background()
	mysqlConfig := containers.MySQL(t)
	database, err := db.New(mysqlConfig.DSN, sqlcomment.Disabled())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	seeder := seed.New(t, database, nil)
	seeder.Seed(ctx)
	workspaceID := seeder.Resources.UserWorkspace.ID
	project := seeder.CreateProject(ctx, seed.CreateProjectRequest{
		ID: uid.New(uid.ProjectPrefix), WorkspaceID: workspaceID,
		Name: "environment-lifecycle", Slug: uid.New("environment-lifecycle"),
	})
	app := seeder.CreateApp(ctx, seed.CreateAppRequest{
		ID: uid.New(uid.AppPrefix), WorkspaceID: workspaceID, ProjectID: project.ID,
		Name: "web", Slug: "web", DefaultBranch: "main",
	})
	source := seeder.CreateEnvironment(ctx, seed.CreateEnvironmentRequest{
		ID: uid.New(uid.EnvironmentPrefix), WorkspaceID: workspaceID,
		ProjectID: project.ID, AppID: app.ID, Slug: "production", Description: "Production",
	})

	regionID := uid.New(uid.RegionPrefix)
	require.NoError(t, database.UpsertRegion(ctx, db.UpsertRegionParams{
		ID: regionID, Name: uid.New("environment-test"), Platform: "test",
	}))
	policyID := uid.New(uid.AutoscalingPolicyPrefix)
	require.NoError(t, database.InsertHorizontalAutoscalingPolicy(ctx, db.InsertHorizontalAutoscalingPolicyParams{
		ID: policyID, WorkspaceID: workspaceID, ReplicasMin: 2, ReplicasMax: 4,
		CpuThreshold: sql.NullInt16{Int16: 70, Valid: true},
		CreatedAt:     time.Now().UnixMilli(), UpdatedAt: sql.NullInt64{Valid: false},
	}))
	require.NoError(t, database.InsertAppRegionalSettingsWithPolicy(ctx, db.InsertAppRegionalSettingsWithPolicyParams{
		WorkspaceID: workspaceID, AppID: app.ID, EnvironmentID: source.ID,
		RegionID: regionID, Replicas: 4,
		HorizontalAutoscalingPolicyID: sql.NullString{String: policyID, Valid: true},
		CreatedAt:                     time.Now().UnixMilli(), UpdatedAt: sql.NullInt64{Valid: false},
	}))
	_, err = database.RW().ExecContext(ctx,
		"UPDATE app_runtime_settings SET cpu_millicores = 750, memory_mib = 1024, outputs = JSON_ARRAY(JSON_OBJECT('kind','worker','name','queue','command',JSON_ARRAY('npm','run','worker'))) WHERE environment_id = ?",
		source.ID,
	)
	require.NoError(t, err)

	auditSvc, err := auditlogs.New(auditlogs.Config{DB: database})
	require.NoError(t, err)
	const bearer = "environment-test-token"
	service := environmentservice.New(environmentservice.Config{
		Database: database, Auditlogs: auditSvc, Bearer: bearer,
	})
	req := connect.NewRequest(&ctrlv1.CreateEnvironmentRequest{
		WorkspaceId: workspaceID, ProjectId: project.ID, AppId: app.ID,
		SourceEnvironmentId: source.ID, Slug: "staging", Description: "Release candidate",
		DeleteProtection: true,
		Actor:              &ctrlv1.ActorInfo{Id: "user_test", Type: ctrlv1.ActorType_ACTOR_TYPE_USER},
	})
	req.Header().Set("Authorization", "Bearer "+bearer)

	response, err := service.CreateEnvironment(ctx, req)
	require.NoError(t, err)
	environmentID := response.Msg.GetId()

	cloned, err := database.FindEnvironmentById(ctx, environmentID)
	require.NoError(t, err)
	require.Equal(t, "staging", cloned.Slug)
	require.Equal(t, "Release candidate", cloned.Description)
	require.True(t, cloned.DeleteProtection.Valid)
	require.True(t, cloned.DeleteProtection.Bool)

	build, err := database.FindAppBuildSettingByAppEnv(ctx, db.FindAppBuildSettingByAppEnvParams{
		AppID: app.ID, EnvironmentID: environmentID,
	})
	require.NoError(t, err)
	require.Equal(t, "Dockerfile", build.Dockerfile.String)

	runtime, err := database.FindAppRuntimeSettingsByAppAndEnv(ctx, db.FindAppRuntimeSettingsByAppAndEnvParams{
		AppID: app.ID, EnvironmentID: environmentID,
	})
	require.NoError(t, err)
	require.Equal(t, int32(750), runtime.AppRuntimeSetting.CpuMillicores)
	require.Equal(t, int32(1024), runtime.AppRuntimeSetting.MemoryMib)
	require.JSONEq(t, `[{"kind":"worker","name":"queue","command":["npm","run","worker"]}]`, string(runtime.AppRuntimeSetting.Outputs))

	regional, err := database.FindAppRegionalSettingsByAppAndEnv(ctx, db.FindAppRegionalSettingsByAppAndEnvParams{
		AppID: app.ID, EnvironmentID: environmentID,
	})
	require.NoError(t, err)
	require.Len(t, regional, 1)
	require.Equal(t, int32(2), regional[0].AutoscalingReplicasMin.Int32)
	require.Equal(t, int32(4), regional[0].AutoscalingReplicasMax.Int32)
	require.NotEqual(t, policyID, regional[0].HorizontalAutoscalingPolicyID.String)

	envVars, err := database.FindAppEnvVarsByAppAndEnv(ctx, db.FindAppEnvVarsByAppAndEnvParams{
		AppID: app.ID, EnvironmentID: environmentID,
	})
	require.NoError(t, err)
	require.Empty(t, envVars, "secrets must not be copied between environments")
}