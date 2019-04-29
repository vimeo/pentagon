package pentagon

import (
	"testing"

	"github.com/vimeo/pentagon/vault"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestReflectorSimple(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()
	vaultClient := vault.NewMock()

	data := map[string]interface{}{
		"foo": "bar",
		"bar": "baz",
	}
	vaultClient.Write("secrets/data/foo", data)

	r := NewReflector(
		vaultClient,
		k8sClient, DefaultNamespace,
		DefaultLabelValue,
	)

	err := r.Reflect(map[string]string{"secrets/data/foo": "foo"})
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	// now get the secret out of k8s
	secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

	secret, err := secrets.Get("foo", metav1.GetOptions{})
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

	if string(secret.Data["foo"]) != "bar" {
		t.Fatalf("foo does not equal bar: %s", string(secret.Data["foo"]))
	}

	if string(secret.Data["bar"]) != "baz" {
		t.Fatalf("bar does not equal baz: %s", string(secret.Data["bar"]))
	}
}

func TestReflectorNoReconcile(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()
	vaultClient := vault.NewMock()

	data := map[string]interface{}{
		"foo": "bar",
		"bar": "baz",
	}

	// write two secrets
	vaultClient.Write("secrets/data/foo1", data)
	vaultClient.Write("secrets/data/foo2", data)

	r := NewReflector(
		vaultClient,
		k8sClient,
		DefaultNamespace,
		DefaultLabelValue,
	)

	// reflect both secrets
	err := r.Reflect(
		map[string]string{
			"secrets/data/foo1": "foo1",
			"secrets/data/foo2": "foo2",
		},
	)
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	// now get the secrets out of k8s
	secrets := k8sClient.CoreV1().Secrets(DefaultNamespace)

	_, err = secrets.Get("foo1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("foo1 should be there: %s", err)
	}

	_, err = secrets.Get("foo2", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("foo2 should be there: %s", err)
	}

	// reflect again, this time without foo2 -- it should still be there
	// and not get reconciled because we're using the default label value.
	err = r.Reflect(
		map[string]string{
			"secrets/data/foo1": "foo1",
		},
	)
	if err != nil {
		t.Fatalf("reflect didn't work the second time: %s", err)
	}

	_, err = secrets.Get("foo1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("foo1 should still be there: %s", err)
	}

	_, err = secrets.Get("foo2", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("foo2 should still be there: %s", err)
	}
}

func TestReflectorWithReconcile(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()
	vaultClient := vault.NewMock()

	data := map[string]interface{}{
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
	_, err := secrets.Create(otherLabel)
	if err != nil {
		t.Fatalf("unable to create other-reflect secret: %s", err)
	}

	r := NewReflector(vaultClient, k8sClient, DefaultNamespace, "test")

	err = r.Reflect(
		map[string]string{
			"secrets/data/foo1": "foo1",
			"secrets/data/foo2": "foo2",
		},
	)
	if err != nil {
		t.Fatalf("reflect didn't work: %s", err)
	}

	s, err := secrets.Get("foo1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("foo1 should be there: %s", err)
	}

	if s.Labels[LabelKey] != "test" {
		t.Fatalf("foo1 pentagon label should have been 'test': %s", s.Labels[LabelKey])
	}

	_, err = secrets.Get("foo2", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("foo2 should be there: %s", err)
	}

	// reflect again, this time without foo2 -- it should still be there
	// and not get reconciled because we're using the default label value.
	err = r.Reflect(
		map[string]string{
			"secrets/data/foo1": "foo1",
		},
	)
	if err != nil {
		t.Fatalf("reflect didn't work the second time: %s", err)
	}

	// foo1 should still be there because it's still in the mapping
	_, err = secrets.Get("foo1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("foo1 should still be there: %s", err)
	}

	// foo2 should have been deleted because it wasn't in the mapping
	_, err = secrets.Get("foo2", metav1.GetOptions{})
	if !errors.IsNotFound(err) {
		t.Fatalf("foo2 should NOT still be there: %s", err)
	}

	// the one with the different label value should still be there
	_, err = secrets.Get("other-reflect", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("other-reflect should still be there: %s", err)
	}
}
