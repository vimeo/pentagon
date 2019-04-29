package vault

import "testing"

func TestWrite(t *testing.T) {
	m := NewMock()
	secret, err := m.Write("test", map[string]interface{}{
		"blah": "blah",
	})

	if err != nil {
		t.Fatalf("write errored: %s", err)
	}

	if data, ok := secret.Data["data"].(map[string]interface{}); ok {
		if data["blah"] != "blah" {
			t.Fatal("data was not equal!")
		}
	} else {
		t.Fatal("inner data wasn't map[string]interface{}")
	}
}

func TestRead(t *testing.T) {
	m := NewMock()
	_, err := m.Write("test", map[string]interface{}{
		"blah": "blah",
	})

	if err != nil {
		t.Fatalf("write errored: %s", err)
	}

	secret, err := m.Read("test")
	if err != nil {
		t.Fatalf("read errored: %s", err)
	}

	if secret == nil {
		t.Fatal("secret was nil")
	}

	if data, ok := secret.Data["data"].(map[string]interface{}); ok {
		if data["blah"] != "blah" {
			t.Fatal("data was not equal!")
		}
	} else {
		t.Fatal("inner data wasn't map[string]interface{}")
	}
}

func TestReadNotFound(t *testing.T) {
	m := NewMock()

	secret, err := m.Read("not-there")
	if secret != nil {
		t.Fatal("secret should be nil")
	}

	if err != nil {
		t.Fatal("err should be nil")
	}
}
