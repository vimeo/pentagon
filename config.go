package pentagon

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

	// Mappings is the map of vault secret names to kubernetes secret names.
	Mappings map[string]string `yaml:"mappings"`
}

// VaultConfig is the vault configuration.
type VaultConfig struct {
	URL      string        `yaml:"url"`
	AuthType VaultAuthType `yaml:"authType"` // token or gcp-default
	Role     string        `yaml:"role"`     // used for non-token auth
	Token    string        `yaml:"token"`
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
