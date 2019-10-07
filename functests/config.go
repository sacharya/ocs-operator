package functests

import (
	"flag"

	deploymanager "github.com/openshift/ocs-operator/pkg/deploy-manager"
)

// TestNamespace is the namespace we run all the tests in.
const TestNamespace = "ocs-functest"

// TestStorageCluster is the name of the storage cluster the test suite installs
const TestStorageCluster = deploymanager.DefaultStorageCluster

// StorageClassRBD is the name of the ceph rbd storage class the test suite installs
const StorageClassRBD = deploymanager.DefaultStorageClassRBD

var OcsRegistryImage string
var LocalStorageRegistryImage string
var UpgradeToOcsRegistryImage string
var UpgradeToLocalStorageRegistryImage string

func init() {
	flag.StringVar(&OcsRegistryImage, "ocs-registry-image", "", "The ocs-registry container image to use in the deployment")
	flag.StringVar(&LocalStorageRegistryImage, "local-storage-registry-image", "", "The local storage registry image to use in the deployment")
	flag.StringVar(&UpgradeToOcsRegistryImage, "upgrade-to-ocs-registry-image", "", "The ocs-registry container image to upgrade to in the deployment")
	flag.StringVar(&UpgradeToLocalStorageRegistryImage, "upgrade-to-local-storage-registry-image", "", "The local storage registry image to upgrade to in the deployment")
}
