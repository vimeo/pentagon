package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/hashicorp/vault/api"
	"github.com/vimeo/pentagon"
	yaml "gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	if len(os.Args) != 2 {
		log.Printf(
			"incorrect number of arguments. need 2, got %d [%#v]",
			len(os.Args),
			os.Args,
		)
		os.Exit(10)
	}

	configFile, err := ioutil.ReadFile(os.Args[1])
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

	reflector := pentagon.NewReflector(
		vaultClient.Logical(),
		k8sClient,
		config.Namespace,
		config.Label,
	)

	err = reflector.Reflect(config.Mappings)
	if err != nil {
		log.Printf("error reflecting vault values into kubernetes: %s", err)
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
	client, err := api.NewClient(&api.Config{
		Address: vaultConfig.URL,
	})

	if err != nil {
		return nil, err
	}

	switch vaultConfig.AuthType {
	case "token":
		client.SetToken(vaultConfig.Token)
	}

	return client, nil
}
