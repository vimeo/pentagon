package vault

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/vault/api"
)

// EngineType is an identifier for an engine
type EngineType string

const (
	// EngineTypeKeyValueV1 is the identifier for version 1 of the key/value engine.
	EngineTypeKeyValueV1 EngineType = "kv"

	// EngineTypeKeyValueV2 is the identifier for version 2 of the key/value engine.
	EngineTypeKeyValueV2 EngineType = "kv-v2"
)

// AllEngineTypes is a slice of all the engine types known to pentagon.
var AllEngineTypes []EngineType

// AuthType is a custom type to represent different Vault authentication
// methods.
type AuthType string

const (
	// AuthTypeToken expects the Token property to be set on the VaultConfig
	// struct with a token to use.
	AuthTypeToken AuthType = "token"

	// AuthTypeGCPDefault expects the Role property of the VaultConfig struct
	// to be populated with the role that vault expects and will use the machine's
	// default service account, running within GCP.
	AuthTypeGCPDefault AuthType = "gcp-default"
)

func init() {
	AllEngineTypes = []EngineType{
		EngineTypeKeyValueV1,
		EngineTypeKeyValueV2,
	}
}

// Logical is a subset of the inner interface that Logical() returns.
// I'm only implementing two methods because that's all I need.
type Logical interface {
	Read(string) (*api.Secret, error)
	Write(string, map[string]any) (*api.Secret, error)
}

// Mock is a mock vault of secrets.
type Mock struct {
	contents     map[string]*api.Secret
	engineMounts map[string]EngineType
	mu           sync.RWMutex // for synchronizing if anyone cares
}

// NewMock returns a new mock vault client.  engineMounts is a map of the path
// prefix to the type of secrets engine that is mounted.
func NewMock(engineMounts map[string]EngineType) *Mock {
	return &Mock{
		contents:     map[string]*api.Secret{},
		engineMounts: engineMounts,
	}
}

// Read reads secrets from the mock vault.
func (m *Mock) Read(path string) (*api.Secret, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// note that the actual vault client returns (nil, nil) when the secret
	// isn't found
	if secret, found := m.contents[path]; found {
		return secret, nil
	}

	return nil, nil
}

// Write writes secrets into the mock vault.
func (m *Mock) Write(
	path string,
	data map[string]any,
) (*api.Secret, error) {

	var secret *api.Secret

	splitPath := strings.Split(path, "/")

	engineType := m.engineMounts[splitPath[0]]
	switch engineType {
	case EngineTypeKeyValueV1:
		secret = &api.Secret{
			Data: data,
		}
	case EngineTypeKeyValueV2:
		secret = &api.Secret{
			Data: map[string]any{
				"data": data,
			},
		}
	default:
		return nil, fmt.Errorf("unknown engine: %s", engineType)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.contents[path] = secret
	return secret, nil
}
