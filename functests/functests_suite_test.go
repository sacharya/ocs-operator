package functests_test

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	tests "github.com/openshift/ocs-operator/functests"
)

func TestTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tests Suite")
}

var _ = BeforeSuite(func() {
	flag.Parse()
	tests.BeforeTestSuiteSetup()
	if tests.UpgradeToOcsRegistryImage != "" {
	    tests.UpgradeTestSuiteSetUp()
	}
})

var _ = AfterSuite(func() {
	tests.AfterTestSuiteCleanup()
})
