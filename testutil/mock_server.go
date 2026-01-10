package testutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tokuhirom/apprun-dedicated-application-provisioner/api"
)

// MockServer is a mock server for testing that implements the ogen Handler interface.
// Currently supports ListClusters and CreateCluster APIs.
type MockServer struct {
	api.UnimplementedHandler

	mu       sync.RWMutex
	clusters map[api.ClusterID]api.ReadClusterDetail

	// Authentication
	expectedToken  string
	expectedSecret string
}

// NewMockServer creates a new mock server with the given authentication credentials.
func NewMockServer(token, secret string) *MockServer {
	return &MockServer{
		clusters:       make(map[api.ClusterID]api.ReadClusterDetail),
		expectedToken:  token,
		expectedSecret: secret,
	}
}

// MockSecurityHandler handles BasicAuth authentication for the mock server.
type MockSecurityHandler struct {
	server *MockServer
}

// HandleBasicAuth validates the BasicAuth credentials.
func (h *MockSecurityHandler) HandleBasicAuth(ctx context.Context, operationName api.OperationName, t api.BasicAuth) (context.Context, error) {
	if t.Username != h.server.expectedToken || t.Password != h.server.expectedSecret {
		return nil, fmt.Errorf("invalid credentials")
	}
	return ctx, nil
}

// CreateCluster creates a new cluster and returns its ID.
func (m *MockServer) CreateCluster(ctx context.Context, req *api.CreateCluster) (*api.CreateClusterResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if cluster name already exists
	for _, c := range m.clusters {
		if c.Name == req.Name {
			return nil, fmt.Errorf("cluster with name %q already exists", req.Name)
		}
	}

	clusterID := api.ClusterID(uuid.New())

	// Convert ports
	ports := make([]api.ReadLoadBalancerPort, len(req.Ports))
	for i, p := range req.Ports {
		ports[i] = api.ReadLoadBalancerPort{
			Port:     p.Port,
			Protocol: api.ReadLoadBalancerPortProtocol(p.Protocol),
		}
	}

	cluster := api.ReadClusterDetail{
		Name:                req.Name,
		ClusterID:           clusterID,
		Ports:               ports,
		ServicePrincipalID:  req.ServicePrincipalID,
		AutoScalingGroups:   []api.ReadAutoScalingGroupSummary{},
		HasLetsEncryptEmail: req.LetsEncryptEmail.IsSet(),
		Created:             int(time.Now().Unix()),
	}

	m.clusters[clusterID] = cluster

	return &api.CreateClusterResponse{
		Cluster: api.CreatedCluster{
			ClusterID: clusterID,
		},
	}, nil
}

// ListClusters returns a list of all clusters.
func (m *MockServer) ListClusters(ctx context.Context, params api.ListClustersParams) (*api.ListClusterResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	clusters := make([]api.ReadClusterDetail, 0, len(m.clusters))
	for _, c := range m.clusters {
		clusters = append(clusters, c)
	}

	return &api.ListClusterResponse{
		Clusters:   clusters,
		NextCursor: api.OptClusterID{},
	}, nil
}

// NewError creates an error response.
func (m *MockServer) NewError(ctx context.Context, err error) *api.ErrorStatusCode {
	return &api.ErrorStatusCode{
		StatusCode: http.StatusInternalServerError,
		Response: api.Error{
			Status: http.StatusInternalServerError,
			Title:  err.Error(),
		},
	}
}

// AddCluster adds a cluster directly to the mock server (for test setup).
func (m *MockServer) AddCluster(cluster api.ReadClusterDetail) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusters[cluster.ClusterID] = cluster
}

// GetClusterByName returns a cluster by name (for test assertions).
func (m *MockServer) GetClusterByName(name string) (api.ReadClusterDetail, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.clusters {
		if c.Name == name {
			return c, true
		}
	}
	return api.ReadClusterDetail{}, false
}

// ClearClusters removes all clusters from the mock server.
func (m *MockServer) ClearClusters() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusters = make(map[api.ClusterID]api.ReadClusterDetail)
}

// ClusterCount returns the number of clusters.
func (m *MockServer) ClusterCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clusters)
}

// StartTestServer starts an HTTP test server with the mock handler.
// Returns the test server and a cleanup function.
func (m *MockServer) StartTestServer() (*httptest.Server, func()) {
	secHandler := &MockSecurityHandler{server: m}
	server, err := api.NewServer(m, secHandler)
	if err != nil {
		panic(fmt.Sprintf("failed to create server: %v", err))
	}

	ts := httptest.NewServer(server)
	return ts, ts.Close
}
