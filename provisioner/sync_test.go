package provisioner

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tokuhirom/apprun-dedicated-application-provisioner/api"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/config"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/testutil"
)

// =============================================================================
// Helper functions
// =============================================================================

func int32Ptr(v int32) *int32 {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

func setupMockServer(t *testing.T, token, secret string) (*testutil.MockServer, *api.Client, func()) {
	mockServer := testutil.NewMockServer(token, secret)
	ts, cleanup := mockServer.StartTestServer()

	client, err := NewClient(ClientConfig{
		AccessToken:       token,
		AccessTokenSecret: secret,
		BaseURL:           ts.URL,
	})
	require.NoError(t, err)

	return mockServer, client, cleanup
}

func createTestCluster(mockServer *testutil.MockServer, name string) api.ClusterID {
	clusterID := api.ClusterID(uuid.New())
	mockServer.AddCluster(api.ReadClusterDetail{
		Name:               name,
		ClusterID:          clusterID,
		ServicePrincipalID: "sp-123",
	})
	return clusterID
}

func createTestApplication(mockServer *testutil.MockServer, clusterID api.ClusterID, name string) api.ApplicationID {
	appID := api.ApplicationID(uuid.New())
	mockServer.AddApplication(api.ReadApplicationDetail{
		ApplicationID:          appID,
		Name:                   name,
		ClusterID:              clusterID,
		ClusterName:            "test-cluster",
		ActiveVersion:          api.NilInt32{Value: 1, Null: false},
		ScalingCooldownSeconds: 60,
	})
	return appID
}

func createTestVersion(mockServer *testutil.MockServer, appID api.ApplicationID, version api.ApplicationVersionNumber, cpu, memory int64) {
	mockServer.AddApplicationVersion(appID, api.ReadApplicationVersionDetail{
		Version:     version,
		CPU:         cpu,
		Memory:      memory,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 2, Set: true},
		Image:       "nginx:latest",
		ExposedPorts: []api.ExposedPort{
			{
				TargetPort:       80,
				LoadBalancerPort: api.NilPort{Value: 443, Null: false},
				UseLetsEncrypt:   true,
				HealthCheck:      api.NilHealthCheck{Null: true},
			},
		},
	})
}

// =============================================================================
// CreatePlan Tests - Cluster Resolution
// =============================================================================

func TestCreatePlan_ClusterNotFound(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()
	_ = mockServer // unused but needed for setup

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "non-existent-cluster",
		Applications: []config.ApplicationConfig{
			{Name: "test-app"},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	assert.Nil(t, plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `cluster "non-existent-cluster" not found`)
}

func TestCreatePlan_ClusterFound(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName:  "my-cluster",
		Applications: []config.ApplicationConfig{},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	assert.Equal(t, "my-cluster", plan.ClusterName)
	assert.Equal(t, uuid.UUID(clusterID), plan.ClusterID)
}

// =============================================================================
// CreatePlan Tests - New Application
// =============================================================================

func TestCreatePlan_NewApplication(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	createTestCluster(mockServer, "my-cluster")

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "new-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(1),
					Image:       "nginx:latest",
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, "new-app", plan.Actions[0].ApplicationName)
	assert.Equal(t, ActionCreate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "Create new application and version")
}

// =============================================================================
// CreatePlan Tests - Existing Application
// =============================================================================

func TestCreatePlan_ExistingApplication_NoVersion(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	// Create application without version
	createTestApplication(mockServer, clusterID, "existing-app")

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(1),
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, "existing-app", plan.Actions[0].ApplicationName)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "Create initial version (no versions exist)")
}

func TestCreatePlan_ExistingApplication_NoChanges(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, "existing-app", plan.Actions[0].ApplicationName)
	assert.Equal(t, ActionNoop, plan.Actions[0].Action)
	assert.Empty(t, plan.Actions[0].Changes)
}

func TestCreatePlan_ExistingApplication_CPUChanged(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         1000, // Changed from 500
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "CPU: 500 -> 1000")
}

func TestCreatePlan_ExistingApplication_MemoryChanged(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      2048, // Changed from 1024
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "Memory: 1024 -> 2048")
}

func TestCreatePlan_ExistingApplication_ScalingModeChanged(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "cpu", // Changed from "manual"
					MinScale:    int32Ptr(1),
					MaxScale:    int32Ptr(5),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "ScalingMode: manual -> cpu")
}

func TestCreatePlan_ExistingApplication_FixedScaleChanged(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(5), // Changed from 2
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "FixedScale: 2 -> 5")
}

func TestCreatePlan_ExistingApplication_ExposedPortsCountChanged(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
						{TargetPort: 8080, LoadBalancerPort: int32Ptr(8443), UseLetsEncrypt: true}, // Added
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "ExposedPorts count: 1 -> 2")
}

func TestCreatePlan_ExistingApplication_EnvCountChanged(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
					Env: []config.EnvVarConfig{
						{Key: "APP_ENV", Value: stringPtr("production"), Secret: false},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "Env count: 0 -> 1")
}

func TestCreatePlan_ExistingApplication_MultipleChanges(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         1000, // Changed
					Memory:      2048, // Changed
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Len(t, plan.Actions[0].Changes, 2)
	assert.Contains(t, plan.Actions[0].Changes, "CPU: 500 -> 1000")
	assert.Contains(t, plan.Actions[0].Changes, "Memory: 1024 -> 2048")
}

func TestCreatePlan_MultipleApplications(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
			{
				Name: "new-app",
				Spec: config.ApplicationSpec{
					CPU:         1000,
					Memory:      2048,
					ScalingMode: "cpu",
					MinScale:    int32Ptr(1),
					MaxScale:    int32Ptr(10),
					Image:       "myapp:v1",
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)

	require.NoError(t, err)
	require.Len(t, plan.Actions, 2)

	// Find actions by name
	actionByName := make(map[string]PlannedAction)
	for _, a := range plan.Actions {
		actionByName[a.ApplicationName] = a
	}

	assert.Equal(t, ActionNoop, actionByName["existing-app"].Action)
	assert.Equal(t, ActionCreate, actionByName["new-app"].Action)
}

// =============================================================================
// Apply Tests
// =============================================================================

func TestApply_CreateApplication(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "new-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					Image:       "nginx:latest",
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	// Create plan first
	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionCreate, plan.Actions[0].Action)

	// Apply the plan with activation
	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: true})
	require.NoError(t, err)

	// Verify the application was created
	app, found := mockServer.GetApplicationByName(clusterID, "new-app")
	require.True(t, found)
	assert.Equal(t, "new-app", app.Name)

	// Verify the version was created and activated
	assert.False(t, app.ActiveVersion.Null)
	assert.Equal(t, int32(1), app.ActiveVersion.Value)
}

func TestApply_UpdateApplication(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         1000, // Changed
					Memory:      2048, // Changed
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	// Create plan
	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)

	// Apply with activation
	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: true})
	require.NoError(t, err)

	// Verify new version was created
	assert.Equal(t, 2, mockServer.VersionCount(appID))

	// Verify the new version has updated values
	newVersion, found := mockServer.GetApplicationVersionByKey(appID, 2)
	require.True(t, found)
	assert.Equal(t, int64(1000), newVersion.CPU)
	assert.Equal(t, int64(2048), newVersion.Memory)
	// Image should be inherited from previous version
	assert.Equal(t, "nginx:latest", newVersion.Image)

	// Verify the application was updated to use new version
	app, _ := mockServer.GetApplicationByName(clusterID, "existing-app")
	assert.Equal(t, int32(2), app.ActiveVersion.Value)
}

func TestApply_NoopApplication(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, ActionNoop, plan.Actions[0].Action)

	// Apply should succeed and not create new versions (Activate doesn't matter for Noop)
	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: false})
	require.NoError(t, err)

	// Verify no new version was created
	assert.Equal(t, 1, mockServer.VersionCount(appID))
}

func TestApply_MultipleApplications_MixedActions(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	existingAppID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, existingAppID, 1, 500, 1024)

	updateAppID := createTestApplication(mockServer, clusterID, "update-app")
	createTestVersion(mockServer, updateAppID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app", // Noop - no changes
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
			{
				Name: "update-app", // Update - CPU changed
				Spec: config.ApplicationSpec{
					CPU:         1000,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
			{
				Name: "new-app", // Create
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      512,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(1),
					Image:       "alpine:latest",
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 8080, LoadBalancerPort: int32Ptr(8443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)
	require.Len(t, plan.Actions, 3)

	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: true})
	require.NoError(t, err)

	// Verify existing-app unchanged
	assert.Equal(t, 1, mockServer.VersionCount(existingAppID))

	// Verify update-app has new version
	assert.Equal(t, 2, mockServer.VersionCount(updateAppID))

	// Verify new-app was created
	newApp, found := mockServer.GetApplicationByName(clusterID, "new-app")
	require.True(t, found)
	assert.Equal(t, int32(1), newApp.ActiveVersion.Value)
}

// =============================================================================
// Image Inheritance Tests
// =============================================================================

func TestApply_ImageInheritedFromExistingVersion(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")

	// Create version with specific image
	mockServer.AddApplicationVersion(appID, api.ReadApplicationVersionDetail{
		Version:     1,
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 2, Set: true},
		Image:       "myapp:v1.0.0", // This should be inherited
		ExposedPorts: []api.ExposedPort{
			{
				TargetPort:       80,
				LoadBalancerPort: api.NilPort{Value: 443, Null: false},
				UseLetsEncrypt:   true,
				HealthCheck:      api.NilHealthCheck{Null: true},
			},
		},
	})

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         1000, // Changed
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					Image:       "different:image", // This should be ignored
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)

	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: true})
	require.NoError(t, err)

	// Verify image was inherited, not changed
	newVersion, found := mockServer.GetApplicationVersionByKey(appID, 2)
	require.True(t, found)
	assert.Equal(t, "myapp:v1.0.0", newVersion.Image) // Should be inherited
	assert.Equal(t, int64(1000), newVersion.CPU)      // Should be updated
}

func TestApply_NewApplication_UsesConfigImage(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "new-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(1),
					Image:       "myapp:v2.0.0", // Should be used for new app
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)

	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: true})
	require.NoError(t, err)

	// Find the new app
	app, found := mockServer.GetApplicationByName(clusterID, "new-app")
	require.True(t, found)

	// Verify image from config was used
	version, found := mockServer.GetApplicationVersionByKey(app.ApplicationID, 1)
	require.True(t, found)
	assert.Equal(t, "myapp:v2.0.0", version.Image)
}

// =============================================================================
// Apply Tests - Without Activation
// =============================================================================

func TestApply_CreateApplication_WithoutActivation(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "new-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					Image:       "nginx:latest",
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)

	// Apply without activation
	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: false})
	require.NoError(t, err)

	// Verify the application was created
	app, found := mockServer.GetApplicationByName(clusterID, "new-app")
	require.True(t, found)
	assert.Equal(t, "new-app", app.Name)

	// Verify the version was created but NOT activated
	assert.Equal(t, 1, mockServer.VersionCount(app.ApplicationID))
	// ActiveVersion should remain null (not set)
	assert.True(t, app.ActiveVersion.Null)
}

func TestApply_UpdateApplication_WithoutActivation(t *testing.T) {
	mockServer, client, cleanup := setupMockServer(t, "test-token", "test-secret")
	defer cleanup()

	clusterID := createTestCluster(mockServer, "my-cluster")
	appID := createTestApplication(mockServer, clusterID, "existing-app")
	createTestVersion(mockServer, appID, 1, 500, 1024)

	provisioner := NewProvisioner(client)
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         1000, // Changed
					Memory:      2048, // Changed
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80, LoadBalancerPort: int32Ptr(443), UseLetsEncrypt: true},
					},
				},
			},
		},
	}

	plan, err := provisioner.CreatePlan(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)

	// Apply without activation
	err = provisioner.Apply(context.Background(), cfg, plan, ApplyOptions{Activate: false})
	require.NoError(t, err)

	// Verify new version was created
	assert.Equal(t, 2, mockServer.VersionCount(appID))

	// Verify the new version has updated values
	newVersion, found := mockServer.GetApplicationVersionByKey(appID, 2)
	require.True(t, found)
	assert.Equal(t, int64(1000), newVersion.CPU)
	assert.Equal(t, int64(2048), newVersion.Memory)

	// Verify the application was NOT updated to use new version
	// (it should still point to version 1)
	app, _ := mockServer.GetApplicationByName(clusterID, "existing-app")
	assert.Equal(t, int32(1), app.ActiveVersion.Value)
}
