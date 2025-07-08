![Go](https://github.com/vimeo/pentagon/workflows/Go/badge.svg)
[![GoDoc](https://godoc.org/github.com/vimeo/pentagon?status.svg)](https://godoc.org/github.com/vimeo/pentagon) [![Go Report Card](https://goreportcard.com/badge/github.com/vimeo/pentagon)](https://goreportcard.com/report/github.com/vimeo/pentagon) 

# Pentagon
Pentagon is a small application designed to run as a Kubernetes CronJob to periodically copy secrets stored in [Vault](https://www.vaultproject.io) or Google Secrets Manager into equivalent [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/), keeping them synchronized.  Naturally, this should be used with care as "standard" Kubernetes Secrets are simply obfuscated as base64-encoded strings.  However, one can and should use more secure methods of securing secrets including Google's [KMS](https://cloud.google.com/kubernetes-engine/docs/how-to/encrypting-secrets) and restricting roles and service accounts appropriately.

## Why not just query Vault?
That's a good question.  If you have a highly-available Vault setup that is stable and performant and you're able to modify your applications to query Vault, that's a completely reasonable approach to take.  If you don't have such a setup, Pentagon provides a way to cache things securely in Kubernetes secrets which can then be provided to applications without directly introducing a Vault dependency.

## Configuration
Pentagon requires a YAML configuration file, the path to which should be passed as the first and only argument to the application.  It is recommended that you store this configuration in a [ConfigMap](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/) and reference it in the CronJob specification.  A sample configuration follows:

```yaml
vault:
  url: <url to vault>
  authType: # "token" or "gcp-default"
  token: <token value> # if authType == "token" is provided
  defaultEngineType: # "kv" or "kv-v2" (currently supported)
  role: "vault role" # if left empty, queries the GCP metadata service
  tls: # optional [tls options](https://godoc.org/github.com/hashicorp/vault/api#TLSConfig)
namespace: <kubernetes namespace for created secrets>
label: <label value to set for the 'pentagon'-created secrets>
mappings:
  # mappings from vault paths to kubernetes secret names
  - vaultPath: secret/data/vault-path
    secretName: k8s-secretname
    vaultEngineType: # optionally "kv" or "kv-v2" to override the defaultEngineType specified above
    secretType: Opaque # optionally - default "Opaque" e.g.: "kubernetes.io/tls"
  # mappings from google secrets manager paths to kubernetes secret names
  - sourceType: gsm
    path: projects/my-project/secrets/my-secret/versions/latest
    secretName: my-secret
```

### Labels and Reconciliation
By default, Pentagon will add a [metadata label](https://godoc.org/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta) with the key `pentagon` and the value `default`.  At the least, this helps identify Pentagon as the creator and maintainer of the secret.

If you set the `label` configuration parameter, you can control the value of the label, allowing multiple Pentagon instances to exist without stepping on each other.  Setting a non-default `label` also enables reconciliation which will cleanup any secrets that were created by Pentagon with a matching label, but are no longer present in the `mappings` configuration.  This provides a simple way to ensure that old secret data does not remain present in your system after its time has passed.

### About Vault Engine Types
Apparently, different Vault secrets engines have slightly different APIs for returning data.  For instance, here is the response for version 1 of the key/value store:

```json
{
  "request_id": "12a0c057-f475-4bbd-6305-e4c07e66805c",
  "lease_id": "",
  "renewable": false,
  "lease_duration": 2764800,
  "data": {
    "foo": "world"
  },
  "wrap_info": null,
  "warnings": null,
  "auth": null
}
```

Notice that the `data` object has the `foo` key embedded directly.  Alternatively, here is the response for version 2 of the key/value store:

```json
{
  "request_id": "78b921ae-79a8-d7e3-da16-336b634fff22",
  "lease_id": "",
  "renewable": false,
  "lease_duration": 0,
  "data": {
    "data": {
      "foo": "world"
    },
    "metadata": {
      "created_time": "2019-10-01T19:36:25.285387Z",
      "deletion_time": "",
      "destroyed": false,
      "version": 1
    }
  },
  "wrap_info": null,
  "warnings": null,
  "auth": null
}
```

Notice the extra `data` element nested inside the outer `data`.  Vault secrets engines can be mounted at arbitrary paths and it does not appear to be possible to reliably detect which engine was used in the API response directly.  In order to properly unwrap the secret data,indicate either `kv` or `kv-v2` as the `vaultEngineType` in the configuration.  In the common case of using only one secrets engine,  simply define the `defaultEngineType` in the `vault` configuration block and the mapping-level `vaultEngineType` will inherit the default.  For compatibility, the unset default value defaults to `kv`.  Note that this differs from the current default that Vault itself uses for the key/value secrets engine.

## Return Values
The application will return 0 on success (when all keys were copied/updated successfully).  A complete list of all possible return values follows:

| Return Value | Description |
| --- | --- |
| 0 | Successfully copied all keys. |
| 10 | Incorrect number of arguments. |
| 20 | Error opening configuration file. |
| 21 | Error parsing YAML configuration file. |
| 22 | Configuration error. |
| 30 | Unable to instantiate vault client. |
| 31 | Unable to instantiate kubernetes client. |
| 32 | Unable to instantiate Google Secrets Manager client. |
| 40 | Error copying keys. |

## Kubernetes Configuration
Pentagon is intended to be run as a cron job to periodically sync keys.  In order to create/update Kubernetes secrets extra permissions are required.  It is recommended to grant those extra permissions to a separate service account which the application will also use.  The following roles is a sample configuration:

```yaml
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: pentagon
spec:
  schedule: "0 15 * * *"
  concurrencyPolicy: Replace
  jobTemplate:
    metadata:
      labels:
        app: pentagon
    spec:
      parallelism: 1
      completions: 1
      template:
        spec:
          serviceAccountName: pentagon # run with a service account that has access to create/update secrets
          terminationGracePeriodSeconds: 10
          restartPolicy: OnFailure
          containers:
          - name: pentagon
            image: vimeo/pentagon:v1.1.0
            args: ["/etc/pentagon/pentagon.yaml"]
            imagePullPolicy: Always
            resources:
              limits:
                cpu: 250m
                memory: 128Mi
              requests:
                cpu: 250m
                memory: 128Mi
            volumeMounts:
                - name: pentagon-config
                  mountPath: /etc/pentagon
                  readOnly: true
          volumes:
              - name: pentagon-config
                configMap:
                  name: pentagon-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: pentagon-config
data:
  pentagon.yaml: |
    vault:
      url: https://vault.address
      authType: gcp-default
      tls: # optional if you have custom requirements
        capath: /etc/cas/custom-root-ca.crt
    label: mapped
    mappings:
      - vaultPath: secret/config/main/foo.key
        secretName: foo-key
      - vaultPath: secret/ssl/tls/domain.com
        secretName: domain.com
        secretType: "kubernetes.io/tls"
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: Role
metadata:
  name: pentagon
rules:
- apiGroups: ["*"]
  resources:
  - secrets
  verbs: ["get", "list", "create", "update", "delete"]
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pentagon
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: RoleBinding
metadata:
  name: pentagon
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: pentagon
subjects:
- kind: ServiceAccount
  name: pentagon
```

## Contributors
Pentagon is a production of Vimeo's Core Services team with lots of support from Vimeo SRE.
* [@sergiosalvatore](https://github.com/sergiosalvatore)
* [@dfinkel](https://github.com/dfinkel)
* [@sachinagada](https://github.com/sachinagada)
