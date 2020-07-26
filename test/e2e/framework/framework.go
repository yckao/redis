/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package framework

import (
	cm "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"k8s.io/client-go/dynamic"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"
	"path/filepath"

	"github.com/appscode/go/crypto/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"gomodules.xyz/cert/certstore"
	ka "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	"kmodules.xyz/client-go/tools/portforward"
	appcat_cs "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1"
)

var (
	DockerRegistry = "kubedbci"
	DBCatalogName  = "5.0.3-v1"
	//DBCatalogName  = "6.0.5"
	Cluster        = true
	WithTLSConfig  = false
)

type Framework struct {
	restConfig        *rest.Config
	kubeClient        kubernetes.Interface
	dbClient          cs.Interface
	kaClient          ka.Interface
	dmClient          dynamic.Interface
	appCatalogClient  appcat_cs.AppcatalogV1alpha1Interface
	tunnel            *portforward.Tunnel
	namespace         string
	name              string
	StorageClass      string
	CertStore         *certstore.CertStore
	certManagerClient cm.Interface
}

func New(
	restConfig *rest.Config,
	kubeClient kubernetes.Interface,
	extClient cs.Interface,
	kaClient ka.Interface,
	dmClient dynamic.Interface,
	appCatalogClient appcat_cs.AppcatalogV1alpha1Interface,
	certManagerClient cm.Interface,
	storageClass string,
) *Framework {
	store, err := certstore.NewCertStore(afero.NewMemMapFs(), filepath.Join("", "pki"))
	Expect(err).NotTo(HaveOccurred())

	err = store.InitCA()
	Expect(err).NotTo(HaveOccurred())
	return &Framework{
		restConfig:        restConfig,
		kubeClient:        kubeClient,
		dbClient:          extClient,
		kaClient:          kaClient,
		dmClient:          dmClient,
		appCatalogClient:  appCatalogClient,
		certManagerClient: certManagerClient,
		name:              "redis-operator",
		namespace:         rand.WithUniqSuffix(api.ResourceSingularRedis),
		StorageClass:      storageClass,
		CertStore:         store,
	}
}

func (f *Framework) Invoke() *Invocation {
	return &Invocation{
		Framework: f,
		app:       rand.WithUniqSuffix("redis-e2e"),
	}
}

func (fi *Invocation) ExtClient() cs.Interface {
	return fi.dbClient
}

func (fi *Invocation) RestConfig() *rest.Config {
	return fi.restConfig
}

type Invocation struct {
	*Framework
	app           string
	testResources []interface{}
}
