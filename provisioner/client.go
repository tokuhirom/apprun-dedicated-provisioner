package provisioner

import (
	"context"
	"net/http"

	"github.com/tokuhirom/apprun-dedicated-provisioner/api"
)

const defaultBaseURL = "https://secure.sakura.ad.jp/cloud/api/apprun-dedicated/1.0"

// securitySource implements api.SecuritySource
type securitySource struct {
	username string
	password string
}

func (s *securitySource) BasicAuth(ctx context.Context, operationName api.OperationName) (api.BasicAuth, error) {
	return api.BasicAuth{
		Username: s.username,
		Password: s.password,
	}, nil
}

// ClientConfig holds the configuration for the API client
type ClientConfig struct {
	// AccessToken is the API access token (UUID)
	AccessToken string
	// AccessTokenSecret is the API access token secret
	AccessTokenSecret string
	// BaseURL is the API base URL (optional, defaults to production)
	BaseURL string
}

// NewClient creates a new API client with the given configuration
func NewClient(cfg ClientConfig) (*api.Client, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	sec := &securitySource{
		username: cfg.AccessToken,
		password: cfg.AccessTokenSecret,
	}

	return api.NewClient(baseURL, sec, api.WithClient(http.DefaultClient))
}
