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
		if m.VaultEngineType == "" {
			t.Fatalf("empty vault engine type for mapping: %+v", m)
		}
	}
}

func TestNoClobber(t *testing.T) {
	c := &Config{
		Label:     "foo",
		Namespace: "bar",
	}

	c.SetDefaults()

	if c.Label != "foo" {
		t.Fatalf("label should still be foo, is %s", c.Label)
	}

	if c.Namespace != "bar" {
		t.Fatalf("namespace should still be bar, is %s", c.Namespace)
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
