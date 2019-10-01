package vault

import (
	"testing"

	"github.com/hashicorp/vault/api"
)

func TestRoundtrip(t *testing.T) {
	for testName, itbl := range map[string]struct {
		key   string
		data  map[string]interface{}
		check func(testing.TB, *api.Secret, error)
	}{
		"simple-kv-1": {
			key: "kv1/test",
			data: map[string]interface{}{
				"blah": "blah",
			},
			check: func(t testing.TB, s *api.Secret, err error) {
				if err != nil {
					t.Fatalf("errored: %s", err)
				}

				if s.Data["blah"] != "blah" {
					t.Fatal("data was not equal!")
				}
			},
		},
		"simple-kv-2": {
			key: "kv2/test",
			data: map[string]interface{}{
				"blah": "blah",
			},
			check: func(t testing.TB, s *api.Secret, err error) {
				if err != nil {
					t.Fatalf("errored: %s", err)
				}

				if w, ok := s.Data["data"].(map[string]interface{}); ok {
					if w["blah"] != "blah" {
						t.Fatal("data was not equal!")
					}
				} else {
					t.Fatal("extra wrapping didn't exist")
				}
			},
		},
	} {
		tbl := itbl
		t.Run(testName, func(t *testing.T) {
			mounting := map[string]EngineType{
				"kv1": EngineTypeKeyValueV1,
				"kv2": EngineTypeKeyValueV2,
			}
			m := NewMock(mounting)
			secret, err := m.Write(tbl.key, tbl.data)
			tbl.check(t, secret, err)

			// now read the same secret out
			secret, err = m.Read(tbl.key)
			tbl.check(t, secret, err)
		})
	}

}

func TestReadNotFound(t *testing.T) {
	m := NewMock(map[string]EngineType{
		"secret": EngineTypeKeyValueV1,
	})

	secret, err := m.Read("secret/not-there")
	if secret != nil {
		t.Fatal("secret should be nil")
	}

	if err != nil {
		t.Fatal("err should be nil")
	}
}
