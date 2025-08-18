package pentagon

import (
	"fmt"
	"log"

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

	// GSM encoded as just raw bytes (default)
	GSMEncodingTypeDefault = "default"

	// GSM encoded as json
	GSMEncodingTypeJSON = "json"
)

// Config describes the configuration for Pentagon.
type Config struct {
	// Vault is the configuration used to connect to vault.
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

		// copy VaultPath to Path for backward compatibility
		if m.Path == "" && m.VaultPath != "" {
			log.Println("WARNING: Use mapping.Path instead of mapping.VaultPath (deprecated)")
			c.Mappings[i].Path = m.VaultPath
		}

		if m.GSMEncodingType == "" {
			c.Mappings[i].GSMEncodingType = GSMEncodingTypeDefault
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
		if m.Path == "" {
			return fmt.Errorf("path should not be empty: %+v", m)
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

// Mapping is a single mapping for a vault secret to a k8s secret.
type Mapping struct {
	// SourceType is the source of a secret: Vault or GSM. Defaults to Vault.
	SourceType string `yaml:"sourceType"`

	// Path is the path to a Vault or GSM secret.
	// GSM secrets can use one of the following forms;
	// - projects/*/secrets/*/versions/*
	// - projects/*/locations/*/secrets/*/versions/*
	Path string `yaml:"path"`

	// [DEPRECATED] VaultPath is the path to a vault secret. Use Path instead.
	VaultPath string `yaml:"vaultPath"`

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

	// GSMEncodingType enables the parsing of JSON secrets with more than one key-value pair when set
	// to 'json'. For the default behavior, simple values, set to 'string'.
	GSMEncodingType string `yaml:"gsmEncodingType"`

	// GSMSecretKeyValue allows you to specify the value of the Kubernetes key to
	// use for this secret's value in cases where gsmEncodingType is *not* json.  If
	// this is unset, the key name will default to the value of secretName.
	GSMSecretKeyValue string `yaml:"gsmSecretKeyValue"`
}
