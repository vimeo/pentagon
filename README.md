# Pentagon
Pentagon is a small application designed to run as a Kubernetes CronJob to periodically copy secrets stored in [Vault](https://www.vaultproject.io) into equivalent [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/), keeping them synchronized.  Naturally, this should be used with care as "standard" Kubernetes Secrets are simply obfuscated as base64-encoded strings.  However, one can and should use more secure methods of securing secrets including Google's KMS and restricting roles and service accounts appropriately.

Use at your own risk...

## Configuration

Pentagon requires a simple YAML configuration file, the path to which should be passed as the first and only argument to the application.  It is recommended that you store this configuration in a [ConfigMap](https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/) and reference it in the CronJob specification.  A sample configuration follows:

```yaml
vault:
      url: <url to vault>
      authType: token # currently only token is allowed
      token: <token value>
    namespace: <kubernetes namespace for created secrets>
    label: <label value to set for the 'pentagon' get for created secrets>
    mappings:
      # mappings from vault paths to kubernetes secret names
      secret/data/vault-path: k8s-secretname
```

## Return Values
The application will return 0 on success (when all keys were copied/updated successfully).  A complete list of all possible return values follows:

| Return Value | Description |
| --- | --- |
| 0 | Successfully copied all keys. |
| 10 | Incorrect number of arguments. |
| 20 | Error opening configuration file. |
| 21 | Error parsing YAML configuration file. |
| 30 | Unable to instantiate vault client. |
| 31 | Unable to instantiate kubernetes client. |
| 40 | Error copying keys. |
