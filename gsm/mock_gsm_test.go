package gsm

import (
	"context"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

func TestMockGSM(t *testing.T) {
	m := NewMockGSM()
	ctx := context.Background()

	// Test regional secrets.
	req1 := &secretmanagerpb.AccessSecretVersionRequest{
		Name: "projects/foo/locations/bar/secrets/baz/versions/3",
	}
	resp1, err := m.AccessSecretVersion(ctx, req1)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp1.Payload.Data) != "foo_bar_baz_3" {
		t.Fatal(err)
	}

	// Test non-regional secrets.
	req2 := &secretmanagerpb.AccessSecretVersionRequest{
		Name: "projects/foo/secrets/bar/versions/latest",
	}
	resp2, err := m.AccessSecretVersion(ctx, req2)
	if err != nil {
		t.Fatal(err)
	}
	if string(resp2.Payload.Data) != "foo_bar_latest" {
		t.Fatal(err)
	}
}
