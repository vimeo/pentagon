package pentagon

import (
	"testing"

	"github.com/vimeo/pentagon/vault"
)

func TestSetDefaults(t *testing.T) {
	c := &Config{
		Vault: VaultConfig{
			AuthType: vault.AuthTypeToken,
		},
		Mappings: []Mapping{
			{
				VaultPath:  "vaultPath",
				SecretName: "theSecret",
			},
		},
	}

	c.SetDefaults()
	if c.Label != DefaultLabelValue {
		t.Fatalf("label should be %s, is %s", DefaultLabelValue, c.Label)
	}

	if c.Namespace != DefaultNamespace {
		t.Fatalf("namespace should be %s, is %s", DefaultNamespace, c.Namespace)
	}

	if c.Vault.DefaultEngineType != vault.EngineTypeKeyValueV1 {
		t.Fatalf("unexpected default engine type: %s", c.Vault.DefaultEngineType)
	}

	for _, m := range c.Mappings {
		if m.SourceType != VaultSourceType {
			t.Fatalf("source type should have defaulted to vault: %+v", m)
		}
		if m.VaultEngineType == "" {
			t.Fatalf("empty vault engine type for mapping: %+v", m)
		}
		if m.SecretType == "" {
			t.Fatalf("empty Kubernetes secret type for mapping: %+v", m)
		}
	}
}

func TestNoClobber(t *testing.T) {
	c := &Config{
		Label:     "foo",
		Namespace: "bar",
		Mappings: []Mapping{
			{
				SecretType: "kubernetes.io/tls",
			},
		},
	}

	c.SetDefaults()

	if c.Label != "foo" {
		t.Fatalf("label should still be foo, is %s", c.Label)
	}

	if c.Namespace != "bar" {
		t.Fatalf("namespace should still be bar, is %s", c.Namespace)
	}

	for _, m := range c.Mappings {
		if m.SecretType != "kubernetes.io/tls" {
			t.Fatalf("Kubernetes secret type should still be 'kubernetes.io/tls', is %s", m.SecretType)
		}
	}
}

func TestValidate(t *testing.T) {
	c := &Config{}
	err := c.Validate()
	if err == nil {
		t.Fatal("configuration should have been invalid")
	}

	c = &Config{
		Mappings: []Mapping{
			{
				VaultPath:  "foo",
				SecretName: "bar",
			},
		},
	}

	err = c.Validate()
	if err != nil {
		t.Fatalf("configuration should have been valid: %s", err)
	}
}

func TestValidSourceTypes(t *testing.T) {
	c := &Config{
		Mappings: []Mapping{
			{SourceType: ""},
			{SourceType: VaultSourceType},
			{SourceType: GSMSourceType},
		},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("mappings should have been valid: %s", err)
	}
}

func TestInvalidSourceType(t *testing.T) {
	c := &Config{
		Mappings: []Mapping{
			{SourceType: "foo"},
		},
	}
	if err := c.Validate(); err == nil {
		t.Fatalf("failed to detect invalid mapping source type")
	}
}
