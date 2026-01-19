package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tokuhirom/apprun-dedicated-provisioner/api"
)

func TestMockServer_CreateCluster(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	req := &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
		Ports: []api.CreateLoadBalancerPort{
			{Port: 443, Protocol: api.CreateLoadBalancerPortProtocolHTTPS},
		},
	}

	resp, err := server.CreateCluster(ctx, req)
	require.NoError(t, err)
	assert.NotEqual(t, api.ClusterID(uuid.Nil), resp.Cluster.ClusterID)
	assert.Equal(t, 1, server.ClusterCount())

	cluster, found := server.GetClusterByName("test-cluster")
	require.True(t, found)
	assert.Equal(t, "test-cluster", cluster.Name)
	assert.Equal(t, "sp-123", cluster.ServicePrincipalID)
}

func TestMockServer_CreateCluster_DuplicateName(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	req := &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	}

	_, err := server.CreateCluster(ctx, req)
	require.NoError(t, err)

	_, err = server.CreateCluster(ctx, req)
	assert.Error(t, err)
}

func TestMockServer_ListClusters(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	resp, err := server.ListClusters(ctx, api.ListClustersParams{})
	require.NoError(t, err)
	assert.Empty(t, resp.Clusters)

	_, err = server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "cluster-1",
		ServicePrincipalID: "sp-1",
	})
	require.NoError(t, err)

	resp, err = server.ListClusters(ctx, api.ListClustersParams{})
	require.NoError(t, err)
	assert.Len(t, resp.Clusters, 1)
	assert.Equal(t, "cluster-1", resp.Clusters[0].Name)
}

func TestMockServer_AddCluster(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")

	clusterID := api.ClusterID(uuid.New())
	server.AddCluster(api.ReadClusterDetail{
		Name:               "pre-added",
		ClusterID:          clusterID,
		ServicePrincipalID: "sp-pre",
	})

	assert.Equal(t, 1, server.ClusterCount())

	cluster, found := server.GetClusterByName("pre-added")
	require.True(t, found)
	assert.Equal(t, clusterID, cluster.ClusterID)
}

func TestMockServer_ClearClusters(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	_, _ = server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test",
		ServicePrincipalID: "sp",
	})
	require.Equal(t, 1, server.ClusterCount())

	server.ClearClusters()
	assert.Equal(t, 0, server.ClusterCount())
}

func TestMockSecurityHandler_HandleBasicAuth(t *testing.T) {
	server := NewMockServer("valid-token", "valid-secret")
	handler := &MockSecurityHandler{server: server}
	ctx := context.Background()

	_, err := handler.HandleBasicAuth(ctx, api.ListClustersOperation, api.BasicAuth{
		Username: "valid-token",
		Password: "valid-secret",
	})
	assert.NoError(t, err)

	_, err = handler.HandleBasicAuth(ctx, api.ListClustersOperation, api.BasicAuth{
		Username: "invalid-token",
		Password: "invalid-secret",
	})
	assert.Error(t, err)
}

func TestMockServer_CreateApplication(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	// First create a cluster
	clusterResp, err := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	require.NoError(t, err)

	// Create an application
	appResp, err := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})
	require.NoError(t, err)
	assert.NotEqual(t, api.ApplicationID(uuid.Nil), appResp.Application.ApplicationID)
	assert.Equal(t, 1, server.ApplicationCount())

	// Verify the application
	app, found := server.GetApplicationByName(clusterResp.Cluster.ClusterID, "test-app")
	require.True(t, found)
	assert.Equal(t, "test-app", app.Name)
	assert.Equal(t, clusterResp.Cluster.ClusterID, app.ClusterID)
	assert.Equal(t, "test-cluster", app.ClusterName)
}

func TestMockServer_CreateApplication_ClusterNotFound(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	_, err := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: api.ClusterID(uuid.New()),
	})
	assert.Error(t, err)
}

func TestMockServer_CreateApplication_DuplicateName(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, err := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	require.NoError(t, err)

	req := &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	}

	_, err = server.CreateApplication(ctx, req)
	require.NoError(t, err)

	_, err = server.CreateApplication(ctx, req)
	assert.Error(t, err)
}

func TestMockServer_ListApplications(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	// Create two clusters
	cluster1, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "cluster-1",
		ServicePrincipalID: "sp-1",
	})
	cluster2, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "cluster-2",
		ServicePrincipalID: "sp-2",
	})

	// Create apps in both clusters
	_, _ = server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "app-1",
		ClusterID: cluster1.Cluster.ClusterID,
	})
	_, _ = server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "app-2",
		ClusterID: cluster2.Cluster.ClusterID,
	})

	// List all applications
	resp, err := server.ListApplications(ctx, api.ListApplicationsParams{})
	require.NoError(t, err)
	assert.Len(t, resp.Applications, 2)

	// List applications for cluster 1 only
	resp, err = server.ListApplications(ctx, api.ListApplicationsParams{
		ClusterID: api.OptClusterID{Value: cluster1.Cluster.ClusterID, Set: true},
	})
	require.NoError(t, err)
	assert.Len(t, resp.Applications, 1)
	assert.Equal(t, "app-1", resp.Applications[0].Name)
}

func TestMockServer_CreateApplicationVersion(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	// Setup: create cluster and application
	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})

	// Create a version
	versionResp, err := server.CreateApplicationVersion(ctx, &api.CreateApplicationVersion{
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 2, Set: true},
		Image:       "nginx:latest",
		ExposedPorts: []api.ExposedPort{
			{TargetPort: 80},
		},
	}, api.CreateApplicationVersionParams{
		ApplicationID: appResp.Application.ApplicationID,
	})

	require.NoError(t, err)
	assert.Equal(t, api.ApplicationVersionNumber(1), versionResp.ApplicationVersion.Version)

	// Verify the version was stored
	assert.Equal(t, 1, server.VersionCount(appResp.Application.ApplicationID))

	version, found := server.GetApplicationVersionByKey(appResp.Application.ApplicationID, 1)
	require.True(t, found)
	assert.Equal(t, int64(500), version.CPU)
	assert.Equal(t, int64(1024), version.Memory)
	assert.Equal(t, "nginx:latest", version.Image)
}

func TestMockServer_CreateApplicationVersion_IncrementingVersionNumber(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})

	req := &api.CreateApplicationVersion{
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 1, Set: true},
		Image:       "nginx:latest",
	}
	params := api.CreateApplicationVersionParams{
		ApplicationID: appResp.Application.ApplicationID,
	}

	v1, _ := server.CreateApplicationVersion(ctx, req, params)
	v2, _ := server.CreateApplicationVersion(ctx, req, params)
	v3, _ := server.CreateApplicationVersion(ctx, req, params)

	assert.Equal(t, api.ApplicationVersionNumber(1), v1.ApplicationVersion.Version)
	assert.Equal(t, api.ApplicationVersionNumber(2), v2.ApplicationVersion.Version)
	assert.Equal(t, api.ApplicationVersionNumber(3), v3.ApplicationVersion.Version)
}

func TestMockServer_ListApplicationVersions(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})

	// Create two versions
	_, _ = server.CreateApplicationVersion(ctx, &api.CreateApplicationVersion{
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 1, Set: true},
		Image:       "nginx:v1",
	}, api.CreateApplicationVersionParams{ApplicationID: appResp.Application.ApplicationID})

	_, _ = server.CreateApplicationVersion(ctx, &api.CreateApplicationVersion{
		CPU:         1000,
		Memory:      2048,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 2, Set: true},
		Image:       "nginx:v2",
	}, api.CreateApplicationVersionParams{ApplicationID: appResp.Application.ApplicationID})

	// List versions
	resp, err := server.ListApplicationVersions(ctx, api.ListApplicationVersionsParams{
		ApplicationID: appResp.Application.ApplicationID,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Versions, 2)
}

func TestMockServer_GetApplicationVersion(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})

	_, _ = server.CreateApplicationVersion(ctx, &api.CreateApplicationVersion{
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeCPU,
		MinScale:    api.OptInt32{Value: 1, Set: true},
		MaxScale:    api.OptInt32{Value: 5, Set: true},
		Image:       "nginx:latest",
	}, api.CreateApplicationVersionParams{ApplicationID: appResp.Application.ApplicationID})

	resp, err := server.GetApplicationVersion(ctx, api.GetApplicationVersionParams{
		ApplicationID: appResp.Application.ApplicationID,
		Version:       1,
	})
	require.NoError(t, err)
	assert.Equal(t, api.ApplicationVersionNumber(1), resp.ApplicationVersion.Version)
	assert.Equal(t, int64(500), resp.ApplicationVersion.CPU)
	assert.Equal(t, api.ScalingModeCPU, resp.ApplicationVersion.ScalingMode)
}

func TestMockServer_GetApplicationVersion_NotFound(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})

	_, err := server.GetApplicationVersion(ctx, api.GetApplicationVersionParams{
		ApplicationID: appResp.Application.ApplicationID,
		Version:       999,
	})
	assert.Error(t, err)
}

func TestMockServer_UpdateApplication(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})

	// Create a version
	_, _ = server.CreateApplicationVersion(ctx, &api.CreateApplicationVersion{
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 3, Set: true},
		Image:       "nginx:latest",
	}, api.CreateApplicationVersionParams{ApplicationID: appResp.Application.ApplicationID})

	// Update application to set active version
	err := server.UpdateApplication(ctx, &api.UpdateApplication{
		ActiveVersion: api.NilInt32{Value: 1, Null: false},
	}, api.UpdateApplicationParams{
		ApplicationID: appResp.Application.ApplicationID,
	})
	require.NoError(t, err)

	// Verify the update
	app, found := server.GetApplicationByName(clusterResp.Cluster.ClusterID, "test-app")
	require.True(t, found)
	assert.False(t, app.ActiveVersion.Null)
	assert.Equal(t, int32(1), app.ActiveVersion.Value)
	assert.Equal(t, int32(3), app.DesiredCount.Value)
}

func TestMockServer_UpdateApplication_ClearActiveVersion(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})

	_, _ = server.CreateApplicationVersion(ctx, &api.CreateApplicationVersion{
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 2, Set: true},
		Image:       "nginx:latest",
	}, api.CreateApplicationVersionParams{ApplicationID: appResp.Application.ApplicationID})

	// Set active version
	_ = server.UpdateApplication(ctx, &api.UpdateApplication{
		ActiveVersion: api.NilInt32{Value: 1, Null: false},
	}, api.UpdateApplicationParams{ApplicationID: appResp.Application.ApplicationID})

	// Clear active version
	err := server.UpdateApplication(ctx, &api.UpdateApplication{
		ActiveVersion: api.NilInt32{Null: true},
	}, api.UpdateApplicationParams{ApplicationID: appResp.Application.ApplicationID})
	require.NoError(t, err)

	app, _ := server.GetApplicationByName(clusterResp.Cluster.ClusterID, "test-app")
	assert.True(t, app.ActiveVersion.Null)
	assert.True(t, app.DesiredCount.Null)
}

func TestMockServer_ClearAll(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")
	ctx := context.Background()

	clusterResp, _ := server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	})
	appResp, _ := server.CreateApplication(ctx, &api.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterResp.Cluster.ClusterID,
	})
	_, _ = server.CreateApplicationVersion(ctx, &api.CreateApplicationVersion{
		CPU:         500,
		Memory:      1024,
		ScalingMode: api.ScalingModeManual,
		FixedScale:  api.OptInt32{Value: 1, Set: true},
		Image:       "nginx:latest",
	}, api.CreateApplicationVersionParams{ApplicationID: appResp.Application.ApplicationID})

	assert.Equal(t, 1, server.ClusterCount())
	assert.Equal(t, 1, server.ApplicationCount())
	assert.Equal(t, 1, server.VersionCount(appResp.Application.ApplicationID))

	server.ClearAll()

	assert.Equal(t, 0, server.ClusterCount())
	assert.Equal(t, 0, server.ApplicationCount())
}
