package functests

import (
	"flag"

	deploymanager "github.com/openshift/ocs-operator/pkg/deploy-manager"
)

// TestNamespace is the namespace we run all the tests in.
const TestNamespace = "ocs-functest"

// TestStorageCluster is the name of the storage cluster the test suite installs
const TestStorageCluster = deploymanager.DefaultStorageClusterName

// StorageClassRBD is the name of the ceph rbd storage class the test suite installs
const StorageClassRBD = deploymanager.DefaultStorageClassRBD

// OcsSubscriptionChannel is the name of the ocs subscription channel
const OcsSubscriptionChannel = "alpha"

// UpgradeToOcsSubscriptionChannel is the name of the ocs subscription channel to upgrade to
const UpgradeToOcsSubscriptionChannel = "beta"

// OcsRegistryImage is the ocs-registry container image to use in the deployment
var OcsRegistryImage string
// LocalStorageRegistryImage is the local storage registry image to use in the deployment
var LocalStorageRegistryImage string
// UpgradeToOcsRegistryImage is the ocs-registry container image to upgrade to in the deployment
var UpgradeToOcsRegistryImage string
// UpgradeToLocalStorageRegistryImage is the local storage registry image to upgrade to in the deployment
var UpgradeToLocalStorageRegistryImage string

func init() {
	flag.StringVar(&OcsRegistryImage, "ocs-registry-image", "", "The ocs-registry container image to use in the deployment")
	flag.StringVar(&LocalStorageRegistryImage, "local-storage-registry-image", "", "The local storage registry image to use in the deployment")
	flag.StringVar(&UpgradeToOcsRegistryImage, "upgrade-to-ocs-registry-image", "", "The ocs-registry container image to upgrade to in the deployment")
	flag.StringVar(&UpgradeToLocalStorageRegistryImage, "upgrade-to-local-storage-registry-image", "", "The local storage registry image to upgrade to in the deployment")
}
