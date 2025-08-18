package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"cloud.google.com/go/compute/metadata"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/hashicorp/vault/api"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/vimeo/pentagon"
	"github.com/vimeo/pentagon/vault"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Printf("caught signal")
		cancel()
		// We're done with this channel, remove the signal handler.
		signal.Stop(sigChan)
	}()

	if len(os.Args) != 2 {
		log.Printf(
			"incorrect number of arguments. need 2, got %d [%#v]",
			len(os.Args),
			os.Args,
		)
		os.Exit(10)
	}

	configFile, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("error opening configuration file: %s", err)
		os.Exit(20)
	}

	config := &pentagon.Config{}
	err = yaml.Unmarshal(configFile, config)
	if err != nil {
		log.Printf("error parsing configuration file: %s", err)
		os.Exit(21)
	}

	config.SetDefaults()

	if err := config.Validate(); err != nil {
		log.Printf("configuration error: %s", err)
		os.Exit(22)
	}

	vaultClient, err := getVaultClient(config.Vault)
	if err != nil {
		log.Printf("unable to get vault client: %s", err)
		os.Exit(30)
	}

	k8sClient, err := getK8sClient()
	if err != nil {
		log.Printf("unable to get kubernetes client: %s", err)
		os.Exit(31)
	}

	gsmClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Printf("unable to get GSM client: %s", err)
		os.Exit(32)
	}
	defer gsmClient.Close()

	reflector := pentagon.NewReflector(
		vaultClient.Logical(),
		gsmClient,
		k8sClient,
		config.Namespace,
		config.Label,
	)

	err = reflector.Reflect(ctx, config.Mappings)
	if err != nil {
		log.Printf("error reflecting secrets into kubernetes: %s", err)
		os.Exit(40)
	}
}

func getK8sClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func getVaultClient(vaultConfig pentagon.VaultConfig) (*api.Client, error) {
	c := api.DefaultConfig()
	c.Address = vaultConfig.URL

	// Set any TLS-specific options for vault if they were provided in the
	// configuration.  The zero-value of the TLSConfig struct should be safe
	// to use anyway.
	if vaultConfig.TLSConfig != nil {
		c.ConfigureTLS(vaultConfig.TLSConfig)
	}

	client, err := api.NewClient(c)
	if err != nil {
		return nil, err
	}

	switch vaultConfig.AuthType {
	case vault.AuthTypeToken:
		client.SetToken(vaultConfig.Token)
	case vault.AuthTypeGCPDefault:
		// default to using configured Role
		role := vaultConfig.Role

		// if that's not provided, get it from the default service account
		if role == "" {
			role, err = getRoleViaGCP()
			if err != nil {
				return nil, fmt.Errorf("error getting role from gcp: %s", err)
			}
		}

		err := setVaultTokenViaGCP(client, role)
		if err != nil {
			return nil, fmt.Errorf("unable to set token via gcp: %s", err)
		}
	default:
		return nil, fmt.Errorf(
			"unsupported vault auth type: %s",
			vaultConfig.AuthType,
		)
	}

	return client, nil
}

func getRoleViaGCP() (string, error) {
	emailAddress, err := metadata.Get("instance/service-accounts/default/email")
	if err != nil {
		return "", fmt.Errorf("error getting default email address: %s", err)
	}
	components := strings.Split(emailAddress, "@")
	return components[0], nil
}

func setVaultTokenViaGCP(vaultClient *api.Client, role string) error {
	// just make a request directly to the metadata server rather
	// than going through the APIs which don't seem to wrap this functionality
	// in a terribly convenient way.
	metadataURL := url.URL{
		Path: "instance/service-accounts/default/identity",
	}

	values := url.Values{}
	vaultAddress, err := url.Parse(vaultClient.Address())
	if err != nil {
		return fmt.Errorf("error parsing vault address: %s", err)
	}
	values.Add(
		"audience",
		fmt.Sprintf("%s/vault/%s", vaultAddress.Hostname(), role),
	)
	values.Add("format", "full")
	metadataURL.RawQuery = values.Encode()

	// `jwt` should be a base64-encoded jwt.
	jwt, err := metadata.Get(metadataURL.String())
	if err != nil {
		return fmt.Errorf("error retrieving JWT from metadata API: %s", err)
	}

	vaultResp, err := vaultClient.Logical().Write(
		"auth/gcp/login",
		map[string]interface{}{
			"role": role,
			"jwt":  jwt,
		},
	)

	if err != nil {
		return fmt.Errorf("error authenticating to vault via gcp: %s", err)
	}

	vaultClient.SetToken(vaultResp.Auth.ClientToken)

	return nil
}
