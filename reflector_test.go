package pentagon

import (
	"context"
	"encoding/json"
	"testing"

	"maps"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/vimeo/pentagon/gsm"
	"github.com/vimeo/pentagon/vault"
)

func allEngineTest(t *testing.T, subTest func(testing.TB, vault.EngineType)) {
	types := vault.AllEngineTypes
	for _, engineType := range types {
		et := engineType
		t.Run(string(engineType), func(innerT *testing.T) {
			innerT.Parallel()
			subTest(innerT, et)
		})
	}
}

func TestReflectorSimple(t *testing.T) {
	allEngineTest(t, func(t testing.TB, engineType vault.EngineType) {
		ctx := context.Background()

		k8sClient := k8sfake.NewSimpleClientset()
		vaultClient := vault.NewMock(map[string]vault.EngineType{
			"secrets": engineType,
		})

		data := map[string]any{
			"foo": "bar",
			"bar": "baz",
		}
		vaultClient.Write("secrets/data/foo", data)

		r := NewReflector(
			vaultClient,
			gsm.NewMockGSM(nil),
			k8sClient, DefaultNamespace,
			DefaultLabelValue,
		)

		err := r.Reflect(ctx, []Mapping{
			{
				SourceType:      "vault",
				Path:            "secrets/data/foo",
				SecretName:      "foo",
				VaultEngineType: engineType,
			},
		})
		if err != nil {
			t.Fatalf("reflect didn't work: %s", err)
		}

		// now get the secret out of k8s
		secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

		secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("secret should be there: %s", err)
		}

		// no additional labels provided. check for default
		if !maps.Equal(secret.Labels, map[string]string{LabelKey: DefaultLabelValue}) {
			t.Fatalf("labels do not match: got %v, want %v", secret.Labels,
				map[string]string{LabelKey: DefaultLabelValue},
			)
		}

		if string(secret.Data["foo"]) != "bar" {
			t.Fatalf("secret value does not equal bar: %s", string(secret.Data["foo"]))
		}

		if string(secret.Data["bar"]) != "baz" {
			t.Fatalf("secret value does not equal baz: %s", string(secret.Data["bar"]))
		}
	})
}

func TestReflectorAdditionalSecretLabelsVault(t *testing.T) {
	allEngineTest(t, func(t testing.TB, engineType vault.EngineType) {
		ctx := context.Background()

		k8sClient := k8sfake.NewSimpleClientset()
		vaultClient := vault.NewMock(map[string]vault.EngineType{
			"secrets": engineType,
		})

		data := map[string]interface{}{
			"foo": "bar",
			"bar": "baz",
		}
		vaultClient.Write("secrets/data/foo", data)

		r := NewReflector(
			vaultClient,
			gsm.NewMockGSM(nil),
			k8sClient, DefaultNamespace,
			DefaultLabelValue,
		)

		err := r.Reflect(ctx, []Mapping{
			{
				SourceType:             "vault",
				Path:                   "secrets/data/foo",
				SecretName:             "foo",
				VaultEngineType:        engineType,
				AdditionalSecretLabels: map[string]string{"secret": "foo"},
			},
		})
		if err != nil {
			t.Fatalf("reflect didn't work: %s", err)
		}

		// now get the secret out of k8s
		secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

		secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("secret should be there: %s", err)
		}

		// check additional labels and default
		if !maps.Equal(secret.Labels, map[string]string{LabelKey: DefaultLabelValue, "secret": "foo"}) {
			t.Fatalf("labels do not match: got %v, want %v", secret.Labels,
				map[string]string{LabelKey: DefaultLabelValue, "secret": "foo"},
			)
		}

		if string(secret.Data["foo"]) != "bar" {
			t.Fatalf("secret value does not equal bar: %s", string(secret.Data["foo"]))
		}

		if string(secret.Data["bar"]) != "baz" {
			t.Fatalf("secret value does not equal baz: %s", string(secret.Data["bar"]))
		}
	})
}

func TestReflectorDefaultLabelOverwriteVault(t *testing.T) {
	allEngineTest(t, func(t testing.TB, engineType vault.EngineType) {
		ctx := context.Background()

		k8sClient := k8sfake.NewSimpleClientset()
		vaultClient := vault.NewMock(map[string]vault.EngineType{
			"secrets": engineType,
		})

		data := map[string]interface{}{
			"foo": "bar",
			"bar": "baz",
		}
		vaultClient.Write("secrets/data/foo", data)

		r := NewReflector(
			vaultClient,
			gsm.NewMockGSM(nil),
			k8sClient, DefaultNamespace,
			DefaultLabelValue,
		)

		err := r.Reflect(ctx, []Mapping{
			{
				SourceType:             "vault",
				Path:                   "secrets/data/foo",
				SecretName:             "foo",
				VaultEngineType:        engineType,
				AdditionalSecretLabels: map[string]string{LabelKey: "wrong-value"},
			},
		})
		if err != nil {
			t.Fatalf("reflect didn't work: %s", err)
		}

		// now get the secret out of k8s
		secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

		secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("secret should be there: %s", err)
		}

		// ensure default `pentagon` label was not overwritten
		if string(secret.Labels[LabelKey]) != DefaultLabelValue {
			t.Fatalf("default pentagon label should be %s is %s", DefaultLabelValue, secret.Labels[LabelKey])
		}
	})
}

func TestReflectorGSM(t *testing.T) {
	ctx := context.Background()
	k8sClient := k8sfake.NewSimpleClientset()

	gsm := gsm.NewMockGSM(map[string][]byte{
		"projects/foo/secrets/bar/versions/latest": []byte("foo_bar_latest"),
	})

	r := NewReflector(
		nil,
		gsm,
		k8sClient, DefaultNamespace,
		DefaultLabelValue,
	)

	err := r.Reflect(ctx, []Mapping{
		{
			SourceType:        "gsm",
			Path:              "projects/foo/secrets/bar/versions/latest",
			SecretName:        "foo",
			GSMSecretKeyValue: "foo-key",
		},
	})
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	// now get the secret out of k8s
	secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

	secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret should be there: %s", err)
	}

	// no additional labels provided. check for default
	if !maps.Equal(secret.Labels, map[string]string{LabelKey: DefaultLabelValue}) {
		t.Fatalf("labels do not match: got %v, want %v", secret.Labels,
			map[string]string{LabelKey: DefaultLabelValue},
		)
	}

	if string(secret.Data["foo-key"]) != "foo_bar_latest" {
		t.Fatalf("secret value does not equal foo_bar_latest: %s", string(secret.Data["foo"]))
	}
}

func TestReflectorAdditionalSecretLabelsGSM(t *testing.T) {
	ctx := context.Background()
	k8sClient := k8sfake.NewSimpleClientset()

	gsm := gsm.NewMockGSM(map[string][]byte{
		"projects/foo/secrets/bar/versions/latest": []byte("foo_bar_latest"),
	})

	r := NewReflector(
		nil,
		gsm,
		k8sClient, DefaultNamespace,
		DefaultLabelValue,
	)

	err := r.Reflect(ctx, []Mapping{
		{
			SourceType:             "gsm",
			Path:                   "projects/foo/secrets/bar/versions/latest",
			SecretName:             "foo",
			GSMSecretKeyValue:      "foo-key",
			AdditionalSecretLabels: map[string]string{"secret": "foo"},
		},
	})
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	// now get the secret out of k8s
	secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

	secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret should be there: %s", err)
	}

	// check additional labels and default
	if !maps.Equal(secret.Labels, map[string]string{LabelKey: DefaultLabelValue, "secret": "foo"}) {
		t.Fatalf("labels do not match: got %v, want %v", secret.Labels,
			map[string]string{LabelKey: DefaultLabelValue, "secret": "foo"},
		)
	}

	// ensure default `pentagon` label was no overwritten
	if string(secret.Labels[LabelKey]) != DefaultLabelValue {
		t.Fatalf("default pentagon label should be %s is %s", DefaultLabelValue, secret.Labels[LabelKey])
	}

	if string(secret.Data["foo-key"]) != "foo_bar_latest" {
		t.Fatalf("secret value does not equal foo_bar_latest: %s", string(secret.Data["foo"]))
	}
}

func TestReflectorDefaultLabelOverwriteGSM(t *testing.T) {
	ctx := context.Background()
	k8sClient := k8sfake.NewSimpleClientset()

	gsm := gsm.NewMockGSM(map[string][]byte{
		"projects/foo/secrets/bar/versions/latest": []byte("foo_bar_latest"),
	})

	r := NewReflector(
		nil,
		gsm,
		k8sClient, DefaultNamespace,
		DefaultLabelValue,
	)

	err := r.Reflect(ctx, []Mapping{
		{
			SourceType:             "gsm",
			Path:                   "projects/foo/secrets/bar/versions/latest",
			SecretName:             "foo",
			GSMSecretKeyValue:      "foo-key",
			AdditionalSecretLabels: map[string]string{LabelKey: "wrong-value"},
		},
	})
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	// now get the secret out of k8s
	secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

	secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret should be there: %s", err)
	}

	// ensure default `pentagon` label was not overwritten
	if string(secret.Labels[LabelKey]) != DefaultLabelValue {
		t.Fatalf("default pentagon label should be %s is %s", DefaultLabelValue, secret.Labels[LabelKey])
	}
}

func TestReflectorGSMJSONStruct(t *testing.T) {
	ctx := context.Background()
	k8sClient := k8sfake.NewSimpleClientset()

	gsm := gsm.NewMockGSM(map[string][]byte{
		"projects/foo/secrets/bar/versions/latest": []byte(`{"key1": {"int": 1, "string": "hello"}, "key2": {"float": 3.14, "bool": true}}`),
	})

	r := NewReflector(
		nil,
		gsm,
		k8sClient, DefaultNamespace,
		DefaultLabelValue,
	)

	err := r.Reflect(ctx, []Mapping{
		{
			SourceType:      "gsm",
			Path:            "projects/foo/secrets/bar/versions/latest",
			GSMEncodingType: GSMEncodingTypeJSON,
			SecretName:      "foo",
		},
	})
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	// now get the secret out of k8s
	secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

	secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret should be there: %s", err)
	}

	if secret.Labels[LabelKey] != DefaultLabelValue {
		t.Fatalf(
			"secret pentagon label should be %s is %s",
			DefaultLabelValue,
			secret.Labels[LabelKey],
		)
	}

	data := map[string]any{}
	if err := json.Unmarshal(secret.Data["key1"], &data); err != nil {
		t.Fatalf("error unmarshaling secret data for key1: %s", err)
	}
	if data["int"] != float64(1) {
		t.Errorf("secret data for key1 does not contain expected int: %+v", data["int"])
	}
	if data["string"] != "hello" {
		t.Errorf("secret data for key1 does not contain expected string: %v", data["string"])
	}

	data = map[string]any{}
	if err := json.Unmarshal(secret.Data["key2"], &data); err != nil {
		t.Fatalf("error unmarshaling secret data for key1: %s", err)
	}

	if data["float"] != 3.14 {
		t.Errorf("secret data for key2 does not contain expected float: %v", data["float"])
	}
	if data["bool"] != true {
		t.Errorf("secret data for key2 does not contain expected bool: %v", data["bool"])
	}
}

func TestReflectorGSMJSONUnwrap(t *testing.T) {
	ctx := context.Background()
	k8sClient := k8sfake.NewSimpleClientset()

	gsm := gsm.NewMockGSM(map[string][]byte{
		"projects/foo/secrets/bar/versions/latest": []byte(`{"key1": 1, "key2": "val2\nval3"}`),
	})

	r := NewReflector(
		nil,
		gsm,
		k8sClient, DefaultNamespace,
		DefaultLabelValue,
	)

	err := r.Reflect(ctx, []Mapping{
		{
			SourceType:      "gsm",
			Path:            "projects/foo/secrets/bar/versions/latest",
			GSMEncodingType: GSMEncodingTypeJSON,
			SecretName:      "foo",
		},
	})
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	// now get the secret out of k8s
	secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

	secret, err := secrets.Get(ctx, "foo", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("secret should be there: %s", err)
	}

	if secret.Labels[LabelKey] != DefaultLabelValue {
		t.Fatalf(
			"secret pentagon label should be %s is %s",
			DefaultLabelValue,
			secret.Labels[LabelKey],
		)
	}

	if string(secret.Data["key1"]) != "1" {
		t.Fatalf("secret value does not equal bare int: %s", string(secret.Data["key1"]))
	}

	if string(secret.Data["key2"]) != "val2\nval3" {
		t.Fatalf("secret value does not equal string: %s", string(secret.Data["key2"]))
	}
}

func TestReflectorNoReconcile(t *testing.T) {
	allEngineTest(t, func(t testing.TB, engineType vault.EngineType) {
		ctx := context.Background()

		k8sClient := k8sfake.NewSimpleClientset()
		vaultClient := vault.NewMock(map[string]vault.EngineType{
			"secrets": engineType,
		})

		data := map[string]any{
			"foo": "bar",
			"bar": "baz",
		}

		// write two secrets
		vaultClient.Write("secrets/data/foo1", data)
		vaultClient.Write("secrets/data/foo2", data)

		r := NewReflector(
			vaultClient,
			gsm.NewMockGSM(nil),
			k8sClient,
			DefaultNamespace,
			DefaultLabelValue,
		)

		// reflect both secrets
		err := r.Reflect(ctx, []Mapping{
			{
				SourceType:      "vault",
				Path:            "secrets/data/foo1",
				SecretName:      "foo1",
				VaultEngineType: engineType,
			},
			{
				SourceType:      "vault",
				Path:            "secrets/data/foo2",
				SecretName:      "foo2",
				VaultEngineType: engineType,
			},
		})
		if err != nil {
			t.Fatalf("reflect didn't work: %s", err)
		}

		// now get the secrets out of k8s
		secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

		_, err = secrets.Get(ctx, "foo1", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("foo1 should be there: %s", err)
		}

		_, err = secrets.Get(ctx, "foo2", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("foo2 should be there: %s", err)
		}

		// reflect again, this time without foo2 -- it should still be there
		// and not get reconciled because we're using the default label value.
		err = r.Reflect(ctx, []Mapping{
			{
				SourceType:      "vault",
				Path:            "secrets/data/foo1",
				SecretName:      "foo1",
				VaultEngineType: engineType,
			},
		})
		if err != nil {
			t.Fatalf("reflect didn't work the second time: %s", err)
		}

		_, err = secrets.Get(ctx, "foo1", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("foo1 should still be there: %s", err)
		}

		_, err = secrets.Get(ctx, "foo2", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("foo2 should still be there: %s", err)
		}
	})
}

func TestReflectorWithReconcile(t *testing.T) {
	allEngineTest(t, func(t testing.TB, engineType vault.EngineType) {
		ctx := context.Background()

		k8sClient := k8sfake.NewSimpleClientset()
		vaultClient := vault.NewMock(map[string]vault.EngineType{
			"secrets": engineType,
		})

		data := map[string]any{
			"foo": "bar",
			"bar": "baz",
		}

		// write two secrets
		vaultClient.Write("secrets/data/foo1", data)
		vaultClient.Write("secrets/data/foo2", data)

		secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

		// make another secret with a different label value so we confirm that
		// it's still there after reconciliation
		otherLabel := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "other-reflect",
				Labels: map[string]string{
					LabelKey: "other",
				},
			},
			Data: map[string][]byte{
				"something": []byte("else"),
			},
		}
		_, err := secrets.Create(ctx, otherLabel, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("unable to create other-reflect secret: %s", err)
		}

		r := NewReflector(vaultClient, gsm.NewMockGSM(nil), k8sClient, DefaultNamespace, "test")

		err = r.Reflect(ctx, []Mapping{
			{
				SourceType:      "vault",
				Path:            "secrets/data/foo1",
				SecretName:      "foo1",
				VaultEngineType: engineType,
			},
			{
				SourceType:      "vault",
				Path:            "secrets/data/foo2",
				SecretName:      "foo2",
				VaultEngineType: engineType,
			},
		})
		if err != nil {
			t.Fatalf("reflect didn't work: %s", err)
		}

		s, err := secrets.Get(ctx, "foo1", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("foo1 should be there: %s", err)
		}

		if s.Labels[LabelKey] != "test" {
			t.Fatalf(
				"foo1 pentagon label should have been 'test': %s",
				s.Labels[LabelKey],
			)
		}

		_, err = secrets.Get(ctx, "foo2", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("foo2 should be there: %s", err)
		}

		// reflect again, this time without foo2 -- it should get reconciled
		// because we're using a non-default label value.
		err = r.Reflect(ctx, []Mapping{
			{
				SourceType:      "vault",
				Path:            "secrets/data/foo1",
				SecretName:      "foo1",
				VaultEngineType: engineType,
			},
		})
		if err != nil {
			t.Fatalf("reflect didn't work the second time: %s", err)
		}

		// foo1 should still be there because it's still in the mapping
		_, err = secrets.Get(ctx, "foo1", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("foo1 should still be there: %s", err)
		}

		// foo2 should have been deleted because it wasn't in the mapping
		// and we're using a non-default label
		_, err = secrets.Get(ctx, "foo2", metav1.GetOptions{})
		if !errors.IsNotFound(err) {
			t.Fatalf("foo2 should NOT still be there: %s", err)
		}

		// the one with the different label value should still be there
		_, err = secrets.Get(ctx, "other-reflect", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("other-reflect should still be there: %s", err)
		}
	})
}

func TestUnsupportedEngineType(t *testing.T) {
	ctx := context.Background()
	k8sClient := k8sfake.NewSimpleClientset()

	vaultClient := vault.NewMock(map[string]vault.EngineType{
		"secrets": vault.EngineTypeKeyValueV2,
	})

	data := map[string]any{
		"foo": "bar",
	}
	vaultClient.Write("secrets/data/foo", data)

	r := NewReflector(
		vaultClient,
		gsm.NewMockGSM(nil),
		k8sClient, DefaultNamespace,
		DefaultLabelValue,
	)

	err := r.Reflect(ctx, []Mapping{
		{
			SourceType:      "vault",
			Path:            "secrets/data/foo",
			SecretName:      "foo",
			VaultEngineType: vault.EngineType("unsupported"),
		},
	})
	if err == nil {
		t.Fatal("expected error from unsupported engine type")
	}
}
