package pentagon

import "testing"

func TestSetDefaults(t *testing.T) {
	c := &Config{}

	c.SetDefaults()
	if c.Label != DefaultLabelValue {
		t.Fatalf("label should be %s, is %s", DefaultLabelValue, c.Label)
	}

	if c.Namespace != DefaultNamespace {
		t.Fatalf("namespace should be %s, is %s", DefaultNamespace, c.Namespace)
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
