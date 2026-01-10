package testutil

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/api"
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
	if err != nil {
		t.Fatalf("CreateCluster failed: %v", err)
	}

	if resp.Cluster.ClusterID == api.ClusterID(uuid.Nil) {
		t.Error("Expected non-nil cluster ID")
	}

	// Verify the cluster was stored
	if server.ClusterCount() != 1 {
		t.Errorf("Expected 1 cluster, got %d", server.ClusterCount())
	}

	// Verify we can retrieve it by name
	cluster, found := server.GetClusterByName("test-cluster")
	if !found {
		t.Error("Expected to find cluster by name")
	}
	if cluster.Name != "test-cluster" {
		t.Errorf("Expected name 'test-cluster', got %q", cluster.Name)
	}
	if cluster.ServicePrincipalID != "sp-123" {
		t.Errorf("Expected service principal 'sp-123', got %q", cluster.ServicePrincipalID)
	}
}

func TestMockServer_CreateCluster_DuplicateName(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")

	ctx := context.Background()
	req := &api.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "sp-123",
	}

	_, err := server.CreateCluster(ctx, req)
	if err != nil {
		t.Fatalf("First CreateCluster failed: %v", err)
	}

	// Try to create another cluster with the same name
	_, err = server.CreateCluster(ctx, req)
	if err == nil {
		t.Error("Expected error for duplicate cluster name")
	}
}

func TestMockServer_ListClusters(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")

	ctx := context.Background()

	// Initially empty
	resp, err := server.ListClusters(ctx, api.ListClustersParams{})
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}
	if len(resp.Clusters) != 0 {
		t.Errorf("Expected 0 clusters, got %d", len(resp.Clusters))
	}

	// Create a cluster
	_, err = server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "cluster-1",
		ServicePrincipalID: "sp-1",
	})
	if err != nil {
		t.Fatalf("CreateCluster failed: %v", err)
	}

	// List should now return 1
	resp, err = server.ListClusters(ctx, api.ListClustersParams{})
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}
	if len(resp.Clusters) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(resp.Clusters))
	}
	if resp.Clusters[0].Name != "cluster-1" {
		t.Errorf("Expected cluster name 'cluster-1', got %q", resp.Clusters[0].Name)
	}
}

func TestMockServer_AddCluster(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")

	clusterID := api.ClusterID(uuid.New())
	server.AddCluster(api.ReadClusterDetail{
		Name:               "pre-added",
		ClusterID:          clusterID,
		ServicePrincipalID: "sp-pre",
	})

	if server.ClusterCount() != 1 {
		t.Errorf("Expected 1 cluster, got %d", server.ClusterCount())
	}

	cluster, found := server.GetClusterByName("pre-added")
	if !found {
		t.Error("Expected to find pre-added cluster")
	}
	if cluster.ClusterID != clusterID {
		t.Error("Cluster ID mismatch")
	}
}

func TestMockServer_ClearClusters(t *testing.T) {
	server := NewMockServer("test-token", "test-secret")

	ctx := context.Background()
	_, _ = server.CreateCluster(ctx, &api.CreateCluster{
		Name:               "test",
		ServicePrincipalID: "sp",
	})

	if server.ClusterCount() != 1 {
		t.Fatalf("Expected 1 cluster before clear")
	}

	server.ClearClusters()

	if server.ClusterCount() != 0 {
		t.Errorf("Expected 0 clusters after clear, got %d", server.ClusterCount())
	}
}

func TestMockSecurityHandler_HandleBasicAuth(t *testing.T) {
	server := NewMockServer("valid-token", "valid-secret")
	handler := &MockSecurityHandler{server: server}

	ctx := context.Background()

	// Valid credentials
	_, err := handler.HandleBasicAuth(ctx, api.ListClustersOperation, api.BasicAuth{
		Username: "valid-token",
		Password: "valid-secret",
	})
	if err != nil {
		t.Errorf("Expected no error for valid credentials, got: %v", err)
	}

	// Invalid credentials
	_, err = handler.HandleBasicAuth(ctx, api.ListClustersOperation, api.BasicAuth{
		Username: "invalid-token",
		Password: "invalid-secret",
	})
	if err == nil {
		t.Error("Expected error for invalid credentials")
	}
}
