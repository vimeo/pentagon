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
				Path:       "path",
				SecretName: "theSecret",
			},
			{
				VaultPath:  "path",
				SecretName: "theSecret",
			},
			{
				SourceType: GSMSourceType,
				Path:       "projects/my-project/secrets/my-secret/versions/latest",
				SecretName: "latestSecret",
			},
			{
				SourceType: GSMSourceType,
				Path:       "projects/my-project/secrets/my-secret/versions/3",
				SecretName: "3secret",
			},
			{
				SourceType: GSMSourceType,
				Path:       "projects/my-project/secrets/my-secret/versions/prod-alias",
				SecretName: "prodAliasSecret",
			},
			{
				SourceType: GSMSourceType,
				Path:       "projects/my-project/secrets/my-secret",
				SecretName: "noVersionSecret",
			},
			{
				SourceType: GSMSourceType,
				Path:       "projects/my-project/secrets/my-secret/",
				SecretName: "noVersionSecretWithTrailingSlash",
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

	if c.Mappings[0].SourceType != VaultSourceType {
		t.Fatalf("source type should have defaulted to vault: %+v", c.Mappings[0].SourceType)
	}

	if c.Mappings[1].SourceType != VaultSourceType {
		t.Fatalf("source type should have defaulted to vault: %+v", c.Mappings[1].SourceType)
	}

	if c.Mappings[0].Path != "path" {
		t.Fatalf("path should be unchanged, is %s", c.Mappings[0].Path)
	}

	for _, m := range c.Mappings {
		if m.Path == "" {
			t.Fatalf("empty path for vault secret: %+v", m)
		}
		if m.VaultEngineType == "" {
			t.Fatalf("empty vault engine type for mapping: %+v", m)
		}
		if m.SecretType == "" {
			t.Fatalf("empty Kubernetes secret type for mapping: %+v", m)
		}
	}

	if c.Mappings[2].Path != "projects/my-project/secrets/my-secret/versions/latest" {
		t.Fatalf("latest GSM path should be unchanged, is %s", c.Mappings[2].Path)
	}

	if c.Mappings[3].Path != "projects/my-project/secrets/my-secret/versions/3" {
		t.Fatalf("versioned GSM path should be unchanged, is %s", c.Mappings[3].Path)
	}

	if c.Mappings[4].Path != "projects/my-project/secrets/my-secret/versions/prod-alias" {
		t.Fatalf("aliased GSM path should be unchanged, is %s", c.Mappings[4].Path)
	}

	if c.Mappings[5].Path != "projects/my-project/secrets/my-secret/versions/latest" {
		t.Fatalf("GSM path without version should have latest suffix, is %s", c.Mappings[5].Path)
	}

	if c.Mappings[6].Path != "projects/my-project/secrets/my-secret/versions/latest" {
		t.Fatalf("GSM path without version (but with trailing slash) should have latest suffix, is %s", c.Mappings[6].Path)
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
				Path:       "foo",
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
			{SourceType: "", Path: "foo"},
			{SourceType: VaultSourceType, Path: "foo"},
			{SourceType: GSMSourceType, Path: "foo"},
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
