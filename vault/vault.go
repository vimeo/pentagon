package vault

import (
	"sync"

	"github.com/hashicorp/vault/api"
)

// Logical is a subset of the inner interface that Logical() returns.
// I'm only implementing two methods because that's all I need.
type Logical interface {
	Read(string) (*api.Secret, error)
	Write(string, map[string]interface{}) (*api.Secret, error)
}

// Mock is a mock vault of secrets.
type Mock struct {
	contents map[string]*api.Secret
	mu       sync.RWMutex // for synchronizing if anyone cares
}

// NewMock returns a new mock vault client.
func NewMock() *Mock {
	return &Mock{
		contents: map[string]*api.Secret{},
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
	data map[string]interface{},
) (*api.Secret, error) {

	secret := &api.Secret{
		Data: data,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.contents[path] = secret
	return secret, nil
}
