package pentagon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/vimeo/pentagon/gsm"
	"github.com/vimeo/pentagon/vault"
)

// LabelKey is the name of label that will be attached to every secret created by pentagon.
const LabelKey = "pentagon"

// NewReflector returns a new reflector
func NewReflector(
	vaultClient vault.Logical,
	gsmClient gsm.SecretAccessor,
	k8sClient kubernetes.Interface,
	k8sNamespace string,
	labelValue string,
) *Reflector {
	return &Reflector{
		vaultClient:   vaultClient,
		gsmClient:     gsmClient,
		secretsClient: k8sClient.CoreV1().Secrets(k8sNamespace),
		k8sNamespace:  k8sNamespace,
		labelValue:    labelValue,
	}
}

// Reflector moves secrets from Vault/GSM to Kubernetes
type Reflector struct {
	vaultClient   vault.Logical
	gsmClient     gsm.SecretAccessor
	secretsClient typedv1.SecretInterface
	k8sNamespace  string
	labelValue    string
	secretsSet    map[string]struct{}
}

// Reflect syncs the values between Vault/GSM and k8s secrets based on the mappings passed.
func (r *Reflector) Reflect(ctx context.Context, mappings []Mapping) error {
	// create a set of existing k8s secrets which were created by pentagon
	secretsList, err := r.secretsClient.List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{LabelKey: r.labelValue}.String(),
	})
	if err != nil {
		return fmt.Errorf("error listing secrets: %s", err)
	}
	r.secretsSet = make(map[string]struct{}, secretsList.Size())
	for _, secret := range secretsList.Items {
		r.secretsSet[secret.ObjectMeta.Name] = struct{}{}
	}

	// make a set of the secrets that we're updating so we can reconcile later.
	touchedSecrets := map[string]struct{}{}

	for _, mapping := range mappings {
		var k8sSecretData map[string][]byte
		var msg string
		switch mapping.SourceType {
		case GSMSourceType:
			var err error
			k8sSecretData, err = r.getGSMSecret(ctx, mapping)
			if err != nil {
				return err
			}
			msg = fmt.Sprintf(
				"reflected GSM secret %s to kubernetes secret %s (type %s)",
				mapping.Path,
				mapping.SecretName,
				mapping.SecretType,
			)
		case VaultSourceType:
			var err error
			k8sSecretData, err = r.getVaultSecret(mapping)
			if err != nil {
				return err
			}
			msg = fmt.Sprintf(
				"reflected vault secret %s to kubernetes secret %s (type %s)",
				mapping.Path,
				mapping.SecretName,
				mapping.SecretType,
			)
		default:
			return fmt.Errorf("unknown secret source type: %s", mapping.SourceType)
		}

		if err := r.createK8sSecret(ctx, mapping, k8sSecretData); err != nil {
			return err
		}

		// record the fact that we updated it
		log.Println(msg)
		touchedSecrets[mapping.SecretName] = struct{}{}
	}

	// if we're not using the default label value, delete any secrets that are no longer in our
	// mappings, but might still exist from previous runs in kubernetes
	if r.labelValue != DefaultLabelValue {
		if err := r.reconcile(ctx, r.secretsSet, touchedSecrets); err != nil {
			return fmt.Errorf("error reconciling: %s", err)
		}
	}

	return nil
}

func (r *Reflector) getVaultSecret(mapping Mapping) (map[string][]byte, error) {
	secretData, err := r.vaultClient.Read(mapping.Path)
	if err != nil {
		return nil, fmt.Errorf("error reading vault key '%s': %s", mapping.Path, err)
	}

	if secretData == nil {
		return nil, fmt.Errorf("secret %s not found", mapping.Path)
	}

	// convert map[string]interface{} to map[string][]byte
	var k8sSecretData map[string][]byte
	switch mapping.VaultEngineType {
	case vault.EngineTypeKeyValueV1:
		k8sSecretData, err = r.castData(secretData.Data)
		if err != nil {
			return nil, fmt.Errorf("error casting data: %s", err)
		}
	case vault.EngineTypeKeyValueV2:
		// there's an extra level of wrapping with the v2 kv secrets engine
		if unwrapped, ok := secretData.Data["data"].(map[string]any); ok {
			k8sSecretData, err = r.castData(unwrapped)
			if err != nil {
				return nil, fmt.Errorf("error casting data: %s", err)
			}
		} else {
			return nil, fmt.Errorf("key/value v2 interface did not have expected extra wrapping")
		}
	default:
		return nil, fmt.Errorf("unknown vault engine type: %q", mapping.VaultEngineType)
	}

	return k8sSecretData, nil
}

func (r *Reflector) getGSMSecret(ctx context.Context, mapping Mapping) (map[string][]byte, error) {
	resp, err := r.gsmClient.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: mapping.Path,
	})
	if err != nil {
		return nil, fmt.Errorf("error accessing GSM secret %q: %w", mapping.Path, err)
	}

	if mapping.GSMEncodingType == GSMEncodingTypeJSON {
		var unmarshaled map[string]json.RawMessage
		if err := json.Unmarshal(resp.Payload.Data, &unmarshaled); err != nil {
			return nil, fmt.Errorf("error unmarshaling GSM JSON secret %q: %w", mapping.Path, err)
		}
		casted := make(map[string][]byte, len(unmarshaled))
		for k, v := range unmarshaled {
			var stringVal string
			if err := json.Unmarshal(v, &stringVal); err == nil {
				casted[k] = []byte(stringVal)
				continue
			}
			casted[k] = v
		}
		return casted, nil
	}

	keyName := mapping.GSMSecretKeyValue
	if keyName == "" {
		keyName = mapping.SecretName
	}

	return map[string][]byte{keyName: resp.Payload.Data}, nil
}

func (r *Reflector) createK8sSecret(ctx context.Context, mapping Mapping, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mapping.SecretName,
			Namespace: r.k8sNamespace,
			Labels:    map[string]string{LabelKey: r.labelValue},
		},
		Data: data,
		Type: mapping.SecretType,
	}

	if _, ok := r.secretsSet[mapping.SecretName]; ok {
		// secret already exists, so we should update it
		_, err := r.secretsClient.Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating secret: %s", err)
		}
	} else {
		// secret doesn't exist, so create it
		_, err := r.secretsClient.Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating secret: %s", err)
		}
	}
	return nil
}

// reconcile deletes any secrets that were not part of the mapping (but still present in the secrets
// with the same label)
func (r *Reflector) reconcile(
	ctx context.Context,
	allSecrets map[string]struct{},
	touchedSecrets map[string]struct{},
) error {
	for secret := range allSecrets {
		if _, found := touchedSecrets[secret]; !found {
			// it was in the list, but we didn't update it (or create it)
			err := r.secretsClient.Delete(ctx, secret, metav1.DeleteOptions{})

			// not found is ok, since we're deleting the secret
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

// castData turns vault map[string]interface{}'s into map[string][]byte's
func (r *Reflector) castData(
	innerData map[string]any,
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
