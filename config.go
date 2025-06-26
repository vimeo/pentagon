package pentagon

import (
	"fmt"

	"github.com/hashicorp/vault/api"
	"github.com/vimeo/pentagon/vault"

	corev1 "k8s.io/api/core/v1"
)

const (
	// DefaultNamespace is the default kubernetes namespace.
	DefaultNamespace = "default"

	// DefaultLabelValue is the default label value that will be applied to secrets
	// created by pentagon.
	DefaultLabelValue = "default"

	// VaultSourceType indicates a mapping sourced from Hashicorp Vault.
	VaultSourceType = "vault"

	// GSMSourceType indicates a mapping sourced from Google Secrets Manager.
	GSMSourceType = "gsm"
)

// Config describes the configuration for vaultofsecrets
type Config struct {
	// Vault is the configuration used to connect to vault.
	Vault VaultConfig `yaml:"vault"`

	// GSM is the configuration used to connect to Google Secrets Manager.
	GSM GSMConfig `yaml:"gsm"`

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

	// default to engine type key/value v1 for backward compatibility
	if c.Vault.DefaultEngineType == "" {
		c.Vault.DefaultEngineType = vault.EngineTypeKeyValueV1
	}

	// set all the underlying mapping engine types to their default
	// if unspecified
	for i, m := range c.Mappings {
		// default to vault source type for backward compatibility
		if m.SourceType == "" {
			c.Mappings[i].SourceType = VaultSourceType
		}

		if m.VaultEngineType == "" {
			c.Mappings[i].VaultEngineType = c.Vault.DefaultEngineType
		}
		if m.SecretType == "" {
			c.Mappings[i].SecretType = corev1.SecretTypeOpaque
		}
	}
}

// Validate checks to make sure that the configuration is valid.
func (c *Config) Validate() error {
	if c.Mappings == nil {
		return fmt.Errorf("no mappings provided")
	}

	validSourceTypes := map[string]struct{}{
		"":              {},
		VaultSourceType: {},
		GSMSourceType:   {},
	}

	for _, m := range c.Mappings {
		if _, ok := validSourceTypes[m.SourceType]; !ok {
			return fmt.Errorf("invalid source type: %+v", m.SourceType)
		}
	}

	return nil
}

// VaultConfig is the vault configuration.
type VaultConfig struct {
	// URL is the url to the vault server.
	URL string `yaml:"url"`

	// AuthType can be "token" or "gcp-default".
	AuthType vault.AuthType `yaml:"authType"`

	// DefaultEngineType is the type of secrets engine used because the API
	// responses may differ based on the engine used.  In particular, K/V v2
	// has an extra layer of data wrapping that differs from v1.
	// Allowed values are "kv" and "kv-v2".
	DefaultEngineType vault.EngineType `yaml:"defaultEngineType"`

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

// GSMConfig is the Google Secrets Manager configuration.
type GSMConfig struct {
	// Will add fields as needed.
}

// Mapping is a single mapping for a vault secret to a k8s secret.
type Mapping struct {
	// SourceType is the source of a secret: Vault or GSM. Defaults to Vault.
	SourceType string `yaml:"sourceType"`

	// VaultPath is the path to a vault secret.
	VaultPath string `yaml:"vaultPath"`

	// GSMPath is the full-qualified path of a GSM secret, including its name and version.
	// For example:
	// - projects/*/secrets/*/versions/*
	// - projects/*/secrets/*/versions/latest
	// - projects/*/locations/*/secrets/*/versions/*
	// - projects/*/locations/*/secrets/*/versions/latest
	GSMPath string `yaml:"gsmPath"`

	// SecretName is the name of the k8s secret that the vault contents should
	// be written to.  Note that this must be a DNS-1123-compatible name and
	// match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*
	SecretName string `yaml:"secretName"`

	// SecretType is a k8s SecretType type (string)
	SecretType corev1.SecretType `yaml:"secretType"`

	// VaultEngineType is the type of secrets engine mounted at the path of this
	// Vault secret.  This specifically overrides the DefaultEngineType
	// specified in VaultConfig.
	VaultEngineType vault.EngineType `yaml:"vaultEngineType"`
}
