package functests

import (
	"flag"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	deploymanager "github.com/openshift/ocs-operator/pkg/deploy-manager"
)

// BeforeTestSuiteSetup is the function called to initialize the test environment
func BeforeTestSuiteSetup() {
	flag.Parse()

	t, err := deploymanager.NewDeployManager()
	gomega.Expect(err).To(gomega.BeNil())

	err = t.CreateNamespace(TestNamespace)
	gomega.Expect(err).To(gomega.BeNil())

	subscriptionChannel := "alpha"
	err = t.DeployOCSWithOLM(OcsRegistryImage, LocalStorageRegistryImage, subscriptionChannel)
	gomega.Expect(err).To(gomega.BeNil())

	err = t.WaitForOCSOperator()
	gomega.Expect(err).To(gomega.BeNil())

	err = t.StartDefaultStorageCluster()
	gomega.Expect(err).To(gomega.BeNil())

}

// AfterTestSuiteCleanup is the function called to tear down the test environment
func AfterTestSuiteCleanup() {
	flag.Parse()

	t, err := deploymanager.NewDeployManager()
	gomega.Expect(err).To(gomega.BeNil())

	err = t.DeleteNamespaceAndWait(TestNamespace)
	gomega.Expect(err).To(gomega.BeNil())

	// TODO uninstall storage cluster.
	// Right now uninstall doesn't work. Once uninstall functions
	// properly, we'll want to uninstall the storage cluster after
	// the testsuite completes
}

// UpgradeTestSuiteSetUp is the function called to upgrade the test environment
func UpgradeTestSuiteSetUp() {
	flag.Parse()

	if UpgradeToOcsRegistryImage == "" || UpgradeToLocalStorageRegistryImage == "" {
		ginkgo.Skip("Condition not met for upgrade")
	}

	t, err := deploymanager.NewDeployManager()
	gomega.Expect(err).To(gomega.BeNil())

	// Get the current csv before the upgrade
	csv, err := t.GetCsv()
	gomega.Expect(err).To(gomega.BeNil())

	subscriptionChannel := "beta"
	err = t.UpgradeOCSWithOLM(UpgradeToOcsRegistryImage, UpgradeToLocalStorageRegistryImage, subscriptionChannel)
	gomega.Expect(err).To(gomega.BeNil())

	err = t.WaitForUpgradeOCSOperator(csv.Name, subscriptionChannel)
	gomega.Expect(err).To(gomega.BeNil())

	// Make sure StorageCluster previously created in the environment is still healthy
	err = t.WaitOnStorageCluster()
	gomega.Expect(err).To(gomega.BeNil())
}
