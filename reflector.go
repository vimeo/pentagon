package pentagon

import (
	"fmt"
	"log"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/vimeo/pentagon/vault"
)

// LabelKey is the name of label that will be attached to every secret created
// by pentagon.
const LabelKey = "pentagon"

// NewReflector returns a new relfector
func NewReflector(
	vaultClient vault.Logical,
	k8sClient kubernetes.Interface,
	k8sNamespace string,
	labelValue string,
) *Reflector {
	return &Reflector{
		vaultClient:  vaultClient,
		k8sClient:    k8sClient,
		k8sNamespace: k8sNamespace,
		labelValue:   labelValue,
	}
}

// Reflector moves things from vault to kuberenetes
type Reflector struct {
	vaultClient  vault.Logical
	k8sClient    kubernetes.Interface
	k8sNamespace string
	labelValue   string
}

// Reflect actually syncs the values between vault and k8s secrets based on
// the mappings passed.
func (r *Reflector) Reflect(mappings map[string]string) error {

	secrets := r.k8sClient.CoreV1().Secrets(r.k8sNamespace)

	// only select secrets that we created
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{LabelKey: r.labelValue}.String(),
	}

	secretsList, err := secrets.List(listOptions)
	if err != nil {
		return fmt.Errorf("error listing secrets: %s", err)
	}

	// make a set of the secrets keyed by name so we can easily access them.
	secretsSet := make(map[string]struct{}, secretsList.Size())
	for _, secret := range secretsList.Items {
		secretsSet[secret.ObjectMeta.Name] = struct{}{}
	}

	// make a set of the secrets that we're actually updating so we can
	// reconcile later.
	touchedSecrets := map[string]struct{}{}

	for vaultKey, k8sKey := range mappings {
		secretData, err := r.vaultClient.Read(vaultKey)
		if err != nil {
			return fmt.Errorf("error reading vault key '%s': %s", vaultKey, err)
		}

		if secretData == nil {
			return fmt.Errorf("secret %s not found", vaultKey)
		}

		// secretData.Data has another "data" map[string]interface{} in here...
		// why? I don't know...
		innerData, ok := secretData.Data["data"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("inner 'data' isn't a map[string]interface{}")
		}

		// convert map[string]interface{} to map[string][]byte
		k8sSecretData, err := r.castData(innerData)
		if err != nil {
			return fmt.Errorf("error casting data: %s", err)
		}

		// create the new Secret
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sKey,
				Namespace: r.k8sNamespace,
				Labels: map[string]string{
					LabelKey: r.labelValue,
				},
			},
			Data: k8sSecretData,
		}

		if _, ok := secretsSet[k8sKey]; ok {
			// secret already exists, so we should update it
			_, err = secrets.Update(newSecret)
			if err != nil {
				return fmt.Errorf("error updating secret: %s", err)
			}
		} else {
			// secret doesn't exist, so create it
			_, err = secrets.Create(newSecret)
			if err != nil {
				return fmt.Errorf("error creating secret: %s", err)
			}
		}

		log.Printf(
			"reflected vault secret %s to kubernetes %s",
			vaultKey,
			k8sKey,
		)

		// record the fact that we actually updated it
		touchedSecrets[newSecret.Name] = struct{}{}
	}

	// if we're not using the default label value, reconcile any secrets
	// that are no longer in vault, but might still exist from previous runs
	// in kubernetes
	if r.labelValue != DefaultLabelValue {
		err = r.reconcile(secretsSet, touchedSecrets)
		if err != nil {
			return fmt.Errorf("error reconciling: %s", err)
		}
	}

	return nil
}

// reconcile delete any secrets that were not part of the mapping (but still
// present in the secrets with the same label)
func (r *Reflector) reconcile(
	allSecrets map[string]struct{},
	touchedSecrets map[string]struct{},
) error {
	secretsAPI := r.k8sClient.CoreV1().Secrets(r.k8sNamespace)

	for secret := range allSecrets {
		if _, found := touchedSecrets[secret]; !found {
			// it was in the list, but we didn't update it (or create it)
			err := secretsAPI.Delete(secret, &metav1.DeleteOptions{})

			// not found is ok because we're deleting, so only return the
			// error if it's NOT not found...
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// castData turns vault map[string]interface{}'s into map[string][]byte's
func (r *Reflector) castData(
	innerData map[string]interface{},
) (map[string][]byte, error) {

	k8sSecretData := make(map[string][]byte, len(innerData))

	for k, v := range innerData {
		switch casted := v.(type) {
		case string:
			k8sSecretData[k] = []byte(casted)
		case []byte:
			k8sSecretData[k] = casted
		default:
			return nil, fmt.Errorf("unknown type of secret %T", v)
		}
	}

	return k8sSecretData, nil
}
