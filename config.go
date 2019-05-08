package pentagon

import (
	"fmt"

	"github.com/hashicorp/vault/api"
)

// DefaultNamespace is the default kubernetes namespace.
const DefaultNamespace = "default"

// DefaultLabelValue is the default label value that will be applied to secrets
// created by pentagon.
const DefaultLabelValue = "default"

// VaultAuthType is a custom type to represent different Vault authentication
// methods.
type VaultAuthType string

// VaultAuthTypeToken expects the Token property to be set on the VaultConfig
// struct with a token to use.
const VaultAuthTypeToken VaultAuthType = "token"

// VaultAuthTypeGCPDefault expects the Role property of the VaultConfig struct
// to be populated with the role that vault expects and will use the machine's
// default service account, running within GCP.
const VaultAuthTypeGCPDefault VaultAuthType = "gcp-default"

// Config describes the configuration for vaultofsecrets
type Config struct {
	// VaultURL is the URL used to connect to vault.
	Vault VaultConfig `yaml:"vault"`

	// Namespace is the k8s namespace that the secrets will be created in.
	Namespace string `yaml:"namespace"`

	// Label is the value of the `pentagon` label that will be added to all
	// k8s secrets created by pentagon.
	Label string `yaml:"label"`

	// Mappings is a list of mappings.
	Mappings []Mapping `yaml:"mappings"`
}

// SetDefaults sets defaults for the Namespace and Label in case they're
// not passed in from the configuration file.
func (c *Config) SetDefaults() {
	if c.Namespace == "" {
		c.Namespace = DefaultNamespace
	}

	if c.Label == "" {
		c.Label = DefaultLabelValue
	}
}

// Validate checks to make sure that the configuration is valid.
func (c *Config) Validate() error {
	if c.Mappings == nil {
		return fmt.Errorf("no mappings provided")
	}

	return nil
}

// VaultConfig is the vault configuration.
type VaultConfig struct {
	// URL is the url to the vault server.
	URL string `yaml:"url"`

	// AuthType can be "token" or "gcp-default".
	AuthType VaultAuthType `yaml:"authType"`

	// Role is the role used when authenticating with vault.  If this is unset
	// the role will be discovered by querying the GCP metadata service for
	// the default service account's email address and using the "user" portion
	// (before the '@').
	Role string `yaml:"role"` // used for non-token auth

	// Token is a vault token and is only considered when AuthType == "token".
	Token string `yaml:"token"`

	// TLSConfig allows you to set any TLS options that the vault client
	// accepts.
	TLSConfig *api.TLSConfig `yaml:"tls"` // for other vault TLS options
}

// Mapping is a single mapping for a vault secret to a k8s secret.
type Mapping struct {
	// VaultPath is the path to the vault secret.
	VaultPath string `yaml:"vaultPath"`

	// SecretName is the name of the k8s secret that the vault contents should
	// be written to.  Note that this must be a DNS-1123-compatible name and
	// match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
	SecretName string `yaml:"secretName"`
}
