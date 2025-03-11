package client

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	clickhousev1 "github.com/altinity/clickhouse-operator/pkg/apis/clickhouse.altinity.com/v1"
	chopClientSet "github.com/altinity/clickhouse-operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type ClickHouseClient struct {
	chopClient chopClientSet.Interface
}

var (
	logger    = utils.GetLogger()
	namespace = utils.GetNamespace()
)

const (
	ClickHouseInstallationCRDResource = "cluster"
)

func userKey(user string) string {
	haskey := "password_sha256_hex" // Fixed string
	// Format the string as "clickhouse/password_sha256_hex"
	passwordKey := fmt.Sprintf("%s/%s", user, haskey)
	return passwordKey
}

// NewClickHouseClient creates a new ClickHouseClient instance
func NewClickHouseClient() (*ClickHouseClient, error) {

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubec", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubec", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		// use the current context in kubeconfig
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}

	}

	chopClient, err := chopClientSet.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ClickHouse client: %v", err)
	}

	return &ClickHouseClient{
		chopClient: chopClient,
	}, nil
}

func (c *ClickHouseClient) getClickHouseInstallation(ctx context.Context) (*clickhousev1.ClickHouseInstallation, error) {
	return c.chopClient.ClickhouseV1().ClickHouseInstallations(namespace).Get(ctx, ClickHouseInstallationCRDResource, metav1.GetOptions{})

}

func (c *ClickHouseClient) UpdateClickhouseUser(ctx context.Context, hashedPassword string, clickhouse_user string) (*clickhousev1.ClickHouseInstallation, error) {

	clickhouseResource, err := c.getClickHouseInstallation(context.Background())
	if err != nil {
		return nil, err
	}

	passwordKey := userKey(clickhouse_user)
	updatedUser := clickhousev1.NewSettingScalar(hashedPassword).SetAttribute(passwordKey, hashedPassword)

	clickhouseResource.Spec.Configuration.Users.Ensure().Set(passwordKey, updatedUser)

	clickhouseResource, err = c.chopClient.ClickhouseV1().ClickHouseInstallations(namespace).Update(ctx, clickhouseResource, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	} else {
		logger.Info("Password updated successfully")
	}
	return clickhouseResource, nil

}
