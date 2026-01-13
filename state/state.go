package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	stateFileSuffix = ".apprun-state.json"
	stateVersion    = 1
)

// ApplicationState holds the state for a single application
type ApplicationState struct {
	RegistryPasswordVersion *int           `json:"registryPasswordVersion,omitempty"`
	SecretEnvVersions       map[string]int `json:"secretEnvVersions,omitempty"`
}

// State represents the state file structure
type State struct {
	Version      int                          `json:"version"`
	Applications map[string]*ApplicationState `json:"applications"`
}

// NewState creates a new empty state
func NewState() *State {
	return &State{
		Version:      stateVersion,
		Applications: make(map[string]*ApplicationState),
	}
}

// GetStatePath returns the state file path based on config file path
// e.g., config.yaml -> config.apprun-state.json
func GetStatePath(configPath string) string {
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	return filepath.Join(dir, name+stateFileSuffix)
}

// LoadState loads the state file from the same directory as config
func LoadState(configPath string) (*State, error) {
	statePath := GetStatePath(configPath)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty state if file doesn't exist
			return NewState(), nil
		}
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	// Initialize map if nil
	if state.Applications == nil {
		state.Applications = make(map[string]*ApplicationState)
	}

	return &state, nil
}

// Save saves the state file to disk
func (s *State) Save(configPath string) error {
	statePath := GetStatePath(configPath)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0644)
}

// GetPasswordVersion returns the stored password version for an application
func (s *State) GetPasswordVersion(appName string) *int {
	if app, ok := s.Applications[appName]; ok {
		return app.RegistryPasswordVersion
	}
	return nil
}

// SetPasswordVersion sets the password version for an application
func (s *State) SetPasswordVersion(appName string, version *int) {
	s.ensureApp(appName)
	s.Applications[appName].RegistryPasswordVersion = version
	s.cleanupApp(appName)
}

// GetSecretEnvVersion returns the stored version for a secret environment variable
func (s *State) GetSecretEnvVersion(appName, envKey string) *int {
	if app, ok := s.Applications[appName]; ok && app.SecretEnvVersions != nil {
		if v, ok := app.SecretEnvVersions[envKey]; ok {
			return &v
		}
	}
	return nil
}

// SetSecretEnvVersion sets the version for a secret environment variable
func (s *State) SetSecretEnvVersion(appName, envKey string, version *int) {
	s.ensureApp(appName)
	if version != nil {
		if s.Applications[appName].SecretEnvVersions == nil {
			s.Applications[appName].SecretEnvVersions = make(map[string]int)
		}
		s.Applications[appName].SecretEnvVersions[envKey] = *version
	} else {
		if s.Applications[appName].SecretEnvVersions != nil {
			delete(s.Applications[appName].SecretEnvVersions, envKey)
		}
	}
	s.cleanupApp(appName)
}

// ensureApp ensures the application state exists
func (s *State) ensureApp(appName string) {
	if _, ok := s.Applications[appName]; !ok {
		s.Applications[appName] = &ApplicationState{}
	}
}

// cleanupApp removes empty application state
func (s *State) cleanupApp(appName string) {
	if app, ok := s.Applications[appName]; ok {
		if app.RegistryPasswordVersion == nil && len(app.SecretEnvVersions) == 0 {
			delete(s.Applications, appName)
		}
	}
}
