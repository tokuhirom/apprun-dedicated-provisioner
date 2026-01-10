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

// ApplicationVersionKey is a key for storing application versions.
type ApplicationVersionKey struct {
	ApplicationID api.ApplicationID
	Version       api.ApplicationVersionNumber
}

// MockServer is a mock server for testing that implements the ogen Handler interface.
// Supports Cluster, Application, and ApplicationVersion APIs used by the provisioner.
type MockServer struct {
	api.UnimplementedHandler

	mu                  sync.RWMutex
	clusters            map[api.ClusterID]api.ReadClusterDetail
	applications        map[api.ApplicationID]api.ReadApplicationDetail
	applicationVersions map[ApplicationVersionKey]api.ReadApplicationVersionDetail
	nextVersionNumber   map[api.ApplicationID]api.ApplicationVersionNumber

	// Authentication
	expectedToken  string
	expectedSecret string
}

// NewMockServer creates a new mock server with the given authentication credentials.
func NewMockServer(token, secret string) *MockServer {
	return &MockServer{
		clusters:            make(map[api.ClusterID]api.ReadClusterDetail),
		applications:        make(map[api.ApplicationID]api.ReadApplicationDetail),
		applicationVersions: make(map[ApplicationVersionKey]api.ReadApplicationVersionDetail),
		nextVersionNumber:   make(map[api.ApplicationID]api.ApplicationVersionNumber),
		expectedToken:       token,
		expectedSecret:      secret,
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

// =============================================================================
// Cluster APIs
// =============================================================================

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

// =============================================================================
// Application APIs
// =============================================================================

// CreateApplication creates a new application.
func (m *MockServer) CreateApplication(ctx context.Context, req *api.CreateApplication) (*api.CreateApplicationResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if cluster exists
	cluster, exists := m.clusters[req.ClusterID]
	if !exists {
		return nil, fmt.Errorf("cluster %s not found", uuid.UUID(req.ClusterID).String())
	}

	// Check if application name already exists in the cluster
	for _, app := range m.applications {
		if app.ClusterID == req.ClusterID && app.Name == req.Name {
			return nil, fmt.Errorf("application with name %q already exists in cluster", req.Name)
		}
	}

	appID := api.ApplicationID(uuid.New())

	app := api.ReadApplicationDetail{
		ApplicationID:          appID,
		Name:                   req.Name,
		ClusterID:              req.ClusterID,
		ClusterName:            cluster.Name,
		ActiveVersion:          api.NilInt32{Null: true},
		DesiredCount:           api.NilInt32{Null: true},
		ScalingCooldownSeconds: 60,
	}

	m.applications[appID] = app
	m.nextVersionNumber[appID] = 1

	return &api.CreateApplicationResponse{
		Application: api.CreatedApplication{
			ApplicationID: appID,
		},
	}, nil
}

// ListApplications returns a list of applications, optionally filtered by cluster.
func (m *MockServer) ListApplications(ctx context.Context, params api.ListApplicationsParams) (*api.ListApplicationResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]api.ReadApplicationDetail, 0)
	for _, app := range m.applications {
		if params.ClusterID.IsSet() && app.ClusterID != params.ClusterID.Value {
			continue
		}
		apps = append(apps, app)
	}

	return &api.ListApplicationResponse{
		Applications: apps,
		NextCursor:   api.OptString{},
	}, nil
}

// =============================================================================
// Application Version APIs
// =============================================================================

// CreateApplicationVersion creates a new version for an application.
func (m *MockServer) CreateApplicationVersion(ctx context.Context, req *api.CreateApplicationVersion, params api.CreateApplicationVersionParams) (*api.CreateApplicationVersionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if application exists
	_, exists := m.applications[params.ApplicationID]
	if !exists {
		return nil, fmt.Errorf("application %s not found", uuid.UUID(params.ApplicationID).String())
	}

	version := m.nextVersionNumber[params.ApplicationID]
	m.nextVersionNumber[params.ApplicationID]++

	// Convert environment variables
	env := make([]api.ReadEnvironmentVariable, len(req.Env))
	for i, e := range req.Env {
		var value api.NilString
		if e.Value.IsSet() {
			value = api.NilString{Value: e.Value.Value, Null: false}
		} else {
			value = api.NilString{Null: true}
		}
		env[i] = api.ReadEnvironmentVariable{
			Key:    e.Key,
			Value:  value,
			Secret: e.Secret,
		}
	}

	versionDetail := api.ReadApplicationVersionDetail{
		Version:           version,
		CPU:               req.CPU,
		Memory:            req.Memory,
		ScalingMode:       req.ScalingMode,
		FixedScale:        req.FixedScale,
		MinScale:          req.MinScale,
		MaxScale:          req.MaxScale,
		ScaleInThreshold:  req.ScaleInThreshold,
		ScaleOutThreshold: req.ScaleOutThreshold,
		Image:             req.Image,
		Cmd:               req.Cmd,
		RegistryUsername:  req.RegistryUsername,
		RegistryPassword:  api.NilString{Null: true}, // Never return password
		ActiveNodeCount:   0,
		Created:           int(time.Now().Unix()),
		ExposedPorts:      req.ExposedPorts,
		Env:               env,
	}

	key := ApplicationVersionKey{
		ApplicationID: params.ApplicationID,
		Version:       version,
	}
	m.applicationVersions[key] = versionDetail

	return &api.CreateApplicationVersionResponse{
		ApplicationVersion: api.ReadApplicationVersionSummary{
			Version: version,
		},
	}, nil
}

// ListApplicationVersions returns a list of versions for an application.
func (m *MockServer) ListApplicationVersions(ctx context.Context, params api.ListApplicationVersionsParams) (*api.ListApplicationVersionResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if application exists
	_, exists := m.applications[params.ApplicationID]
	if !exists {
		return nil, fmt.Errorf("application %s not found", uuid.UUID(params.ApplicationID).String())
	}

	versions := make([]api.ApplicationVersionDeploymentStatus, 0)
	for key, v := range m.applicationVersions {
		if key.ApplicationID == params.ApplicationID {
			versions = append(versions, api.ApplicationVersionDeploymentStatus{
				Version:         v.Version,
				Image:           v.Image,
				ActiveNodeCount: v.ActiveNodeCount,
				Created:         v.Created,
			})
		}
	}

	return &api.ListApplicationVersionResponse{
		Versions:   versions,
		NextCursor: api.OptApplicationVersionNumber{},
	}, nil
}

// GetApplicationVersion returns the details of a specific application version.
func (m *MockServer) GetApplicationVersion(ctx context.Context, params api.GetApplicationVersionParams) (*api.GetApplicationVersionResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := ApplicationVersionKey{
		ApplicationID: params.ApplicationID,
		Version:       params.Version,
	}

	version, exists := m.applicationVersions[key]
	if !exists {
		return nil, fmt.Errorf("version %d not found for application %s", params.Version, uuid.UUID(params.ApplicationID).String())
	}

	return &api.GetApplicationVersionResponse{
		ApplicationVersion: version,
	}, nil
}

// UpdateApplication updates an application (e.g., sets the active version).
func (m *MockServer) UpdateApplication(ctx context.Context, req *api.UpdateApplication, params api.UpdateApplicationParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.applications[params.ApplicationID]
	if !exists {
		return fmt.Errorf("application %s not found", uuid.UUID(params.ApplicationID).String())
	}

	// Update active version
	app.ActiveVersion = req.ActiveVersion

	// Update desired count based on active version
	if !req.ActiveVersion.Null {
		key := ApplicationVersionKey{
			ApplicationID: params.ApplicationID,
			Version:       api.ApplicationVersionNumber(req.ActiveVersion.Value),
		}
		if version, ok := m.applicationVersions[key]; ok {
			if version.ScalingMode == api.ScalingModeManual {
				if version.FixedScale.IsSet() {
					app.DesiredCount = api.NilInt32{Value: version.FixedScale.Value, Null: false}
				}
			} else {
				if version.MinScale.IsSet() {
					app.DesiredCount = api.NilInt32{Value: version.MinScale.Value, Null: false}
				}
			}
		}
	} else {
		app.DesiredCount = api.NilInt32{Null: true}
	}

	m.applications[params.ApplicationID] = app
	return nil
}

// =============================================================================
// Error Handling
// =============================================================================

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

// =============================================================================
// Helper Methods for Testing
// =============================================================================

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

// AddApplication adds an application directly to the mock server (for test setup).
func (m *MockServer) AddApplication(app api.ReadApplicationDetail) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applications[app.ApplicationID] = app
	if _, exists := m.nextVersionNumber[app.ApplicationID]; !exists {
		m.nextVersionNumber[app.ApplicationID] = 1
	}
}

// GetApplicationByName returns an application by name and cluster ID (for test assertions).
func (m *MockServer) GetApplicationByName(clusterID api.ClusterID, name string) (api.ReadApplicationDetail, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, app := range m.applications {
		if app.ClusterID == clusterID && app.Name == name {
			return app, true
		}
	}
	return api.ReadApplicationDetail{}, false
}

// ClearApplications removes all applications from the mock server.
func (m *MockServer) ClearApplications() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applications = make(map[api.ApplicationID]api.ReadApplicationDetail)
	m.applicationVersions = make(map[ApplicationVersionKey]api.ReadApplicationVersionDetail)
	m.nextVersionNumber = make(map[api.ApplicationID]api.ApplicationVersionNumber)
}

// ApplicationCount returns the number of applications.
func (m *MockServer) ApplicationCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.applications)
}

// AddApplicationVersion adds an application version directly to the mock server (for test setup).
func (m *MockServer) AddApplicationVersion(appID api.ApplicationID, version api.ReadApplicationVersionDetail) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := ApplicationVersionKey{
		ApplicationID: appID,
		Version:       version.Version,
	}
	m.applicationVersions[key] = version
	if m.nextVersionNumber[appID] <= version.Version {
		m.nextVersionNumber[appID] = version.Version + 1
	}
}

// GetApplicationVersion returns an application version (for test assertions).
func (m *MockServer) GetApplicationVersionByKey(appID api.ApplicationID, version api.ApplicationVersionNumber) (api.ReadApplicationVersionDetail, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := ApplicationVersionKey{
		ApplicationID: appID,
		Version:       version,
	}
	v, exists := m.applicationVersions[key]
	return v, exists
}

// VersionCount returns the number of versions for an application.
func (m *MockServer) VersionCount(appID api.ApplicationID) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for key := range m.applicationVersions {
		if key.ApplicationID == appID {
			count++
		}
	}
	return count
}

// ClearAll removes all data from the mock server.
func (m *MockServer) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusters = make(map[api.ClusterID]api.ReadClusterDetail)
	m.applications = make(map[api.ApplicationID]api.ReadApplicationDetail)
	m.applicationVersions = make(map[ApplicationVersionKey]api.ReadApplicationVersionDetail)
	m.nextVersionNumber = make(map[api.ApplicationID]api.ApplicationVersionNumber)
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
