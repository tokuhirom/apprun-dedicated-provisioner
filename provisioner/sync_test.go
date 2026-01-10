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

func TestProvisioner_CreatePlan_ClusterNotFound(t *testing.T) {
	// Setup mock server
	mockServer := testutil.NewMockServer("test-token", "test-secret")
	ts, cleanup := mockServer.StartTestServer()
	defer cleanup()

	// Create client pointing to mock server
	client, err := NewClient(ClientConfig{
		AccessToken:       "test-token",
		AccessTokenSecret: "test-secret",
		BaseURL:           ts.URL,
	})
	require.NoError(t, err)

	// Create provisioner
	provisioner := NewProvisioner(client)

	// Create config that references a non-existent cluster
	cfg := &config.ClusterConfig{
		ClusterName: "non-existent-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "test-app",
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

	// Try to create plan - should fail because cluster doesn't exist
	ctx := context.Background()
	plan, err := provisioner.CreatePlan(ctx, cfg)

	// Verify error
	assert.Nil(t, plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster \"non-existent-cluster\" not found")
}

func TestProvisioner_CreatePlan_ClusterFound(t *testing.T) {
	// Setup mock server with a cluster
	mockServer := testutil.NewMockServer("test-token", "test-secret")

	clusterID := api.ClusterID(uuid.New())
	mockServer.AddCluster(api.ReadClusterDetail{
		Name:               "my-cluster",
		ClusterID:          clusterID,
		ServicePrincipalID: "sp-123",
	})

	ts, cleanup := mockServer.StartTestServer()
	defer cleanup()

	// Create client pointing to mock server
	client, err := NewClient(ClientConfig{
		AccessToken:       "test-token",
		AccessTokenSecret: "test-secret",
		BaseURL:           ts.URL,
	})
	require.NoError(t, err)

	// Create provisioner
	provisioner := NewProvisioner(client)

	// Create config that references the existing cluster
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "test-app",
				Spec: config.ApplicationSpec{
					CPU:         500,
					Memory:      1024,
					ScalingMode: "manual",
					FixedScale:  int32Ptr(1),
					Image:       "nginx:latest",
					ExposedPorts: []config.ExposedPortConfig{
						{TargetPort: 80},
					},
				},
			},
		},
	}

	// Create plan - should succeed
	ctx := context.Background()
	plan, err := provisioner.CreatePlan(ctx, cfg)

	// Verify success
	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Equal(t, "my-cluster", plan.ClusterName)
	assert.Equal(t, uuid.UUID(clusterID), plan.ClusterID)

	// Should have one action to create the new app
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, "test-app", plan.Actions[0].ApplicationName)
	assert.Equal(t, ActionCreate, plan.Actions[0].Action)
}

func TestProvisioner_CreatePlan_ExistingApplicationNoChanges(t *testing.T) {
	// Setup mock server with cluster and existing application
	mockServer := testutil.NewMockServer("test-token", "test-secret")

	clusterID := api.ClusterID(uuid.New())
	mockServer.AddCluster(api.ReadClusterDetail{
		Name:               "my-cluster",
		ClusterID:          clusterID,
		ServicePrincipalID: "sp-123",
	})

	appID := api.ApplicationID(uuid.New())
	mockServer.AddApplication(api.ReadApplicationDetail{
		ApplicationID:          appID,
		Name:                   "existing-app",
		ClusterID:              clusterID,
		ClusterName:            "my-cluster",
		ActiveVersion:          api.NilInt32{Value: 1, Null: false},
		ScalingCooldownSeconds: 60,
	})

	// Add a version for the existing app
	mockServer.AddApplicationVersion(appID, api.ReadApplicationVersionDetail{
		Version:     1,
		CPU:         500,
		Memory:      1024,
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

	ts, cleanup := mockServer.StartTestServer()
	defer cleanup()

	client, err := NewClient(ClientConfig{
		AccessToken:       "test-token",
		AccessTokenSecret: "test-secret",
		BaseURL:           ts.URL,
	})
	require.NoError(t, err)

	provisioner := NewProvisioner(client)

	// Config matches existing state
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
					Image:       "nginx:latest",
					ExposedPorts: []config.ExposedPortConfig{
						{
							TargetPort:       80,
							LoadBalancerPort: int32Ptr(443),
							UseLetsEncrypt:   true,
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	plan, err := provisioner.CreatePlan(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, "existing-app", plan.Actions[0].ApplicationName)
	assert.Equal(t, ActionNoop, plan.Actions[0].Action)
}

func TestProvisioner_CreatePlan_ExistingApplicationWithChanges(t *testing.T) {
	// Setup mock server with cluster and existing application
	mockServer := testutil.NewMockServer("test-token", "test-secret")

	clusterID := api.ClusterID(uuid.New())
	mockServer.AddCluster(api.ReadClusterDetail{
		Name:               "my-cluster",
		ClusterID:          clusterID,
		ServicePrincipalID: "sp-123",
	})

	appID := api.ApplicationID(uuid.New())
	mockServer.AddApplication(api.ReadApplicationDetail{
		ApplicationID:          appID,
		Name:                   "existing-app",
		ClusterID:              clusterID,
		ClusterName:            "my-cluster",
		ActiveVersion:          api.NilInt32{Value: 1, Null: false},
		ScalingCooldownSeconds: 60,
	})

	// Add a version for the existing app
	mockServer.AddApplicationVersion(appID, api.ReadApplicationVersionDetail{
		Version:     1,
		CPU:         500,
		Memory:      1024,
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

	ts, cleanup := mockServer.StartTestServer()
	defer cleanup()

	client, err := NewClient(ClientConfig{
		AccessToken:       "test-token",
		AccessTokenSecret: "test-secret",
		BaseURL:           ts.URL,
	})
	require.NoError(t, err)

	provisioner := NewProvisioner(client)

	// Config has different CPU and Memory
	cfg := &config.ClusterConfig{
		ClusterName: "my-cluster",
		Applications: []config.ApplicationConfig{
			{
				Name: "existing-app",
				Spec: config.ApplicationSpec{
					CPU:         1000, // Changed from 500
					Memory:      2048, // Changed from 1024
					ScalingMode: "manual",
					FixedScale:  int32Ptr(2),
					Image:       "nginx:latest",
					ExposedPorts: []config.ExposedPortConfig{
						{
							TargetPort:       80,
							LoadBalancerPort: int32Ptr(443),
							UseLetsEncrypt:   true,
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	plan, err := provisioner.CreatePlan(ctx, cfg)

	require.NoError(t, err)
	require.NotNil(t, plan)
	require.Len(t, plan.Actions, 1)
	assert.Equal(t, "existing-app", plan.Actions[0].ApplicationName)
	assert.Equal(t, ActionUpdate, plan.Actions[0].Action)
	assert.Contains(t, plan.Actions[0].Changes, "CPU: 500 -> 1000")
	assert.Contains(t, plan.Actions[0].Changes, "Memory: 1024 -> 2048")
}

func int32Ptr(v int32) *int32 {
	return &v
}
