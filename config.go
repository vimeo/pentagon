package pentagon

// DefaultNamespace is the default kubernetes namespace.
const DefaultNamespace = "default"

// DefaultLabelValue is the default label value that will be applied to secrets
// created by pentagon.
const DefaultLabelValue = "default"

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
	URL      string `yaml:"url"`
	AuthType string `yaml:"authType"`
	Token    string `yaml:"token"`
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
