package functests_test

import (
	"fmt"
	"time"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ocsv1 "github.com/openshift/ocs-operator/pkg/apis/ocs/v1"
	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
	//storagev1 "k8s.io/api/storage/v1"
	"github.com/openshift/ocs-operator/pkg/controller/util"

	//cv1 "github.com/rook/rook/pkg/client/clientset/versioned/typed/ceph.rook.io/v1"

	deploymanager "github.com/openshift/ocs-operator/pkg/deploy-manager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

var _ = Describe("StorageClusterInit", func() {
	var ocsClient *rest.RESTClient
	//var rookCephClient *rest.RESTClient
	var parameterCodec runtime.ParameterCodec
	var namespace string

	BeforeEach(func() {
		RegisterFailHandler(Fail)
		namespace = deploymanager.InstallNamespace
	})

	Describe("StorageClusterInit", func() {

		Context("Initially", func() {
			FIt("Verify it is created", func() {
				deployManager, err := deploymanager.NewDeployManager()
				Expect(err).To(BeNil())
				ocsClient = deployManager.GetOcsClient()
				parameterCodec = deployManager.GetParameterCodec()
				scis := &ocsv1.StorageClusterInitializationList{}
				err = ocsClient.Get().
						Resource("storageclusterinitializations").
						Namespace(namespace).
						VersionedParams(&metav1.ListOptions{}, parameterCodec).
						Do().
						Into(scis)
				Expect(err).To(BeNil())
				Expect(len(scis.Items)).To(Equal(1))
				Expect(scis.Items[0].Status.Phase).To(Equal(util.PhaseReady))

				cobs := &cephv1.CephObjectStoreList{}
				cobs, err = deployManager.GetRookCephClient().
						CephObjectStores(namespace).
						List(metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(len(cobs.Items)).To(Equal(1))
				Expect(cobs.Items[0].Name).To(Equal(fmt.Sprintf("%s-cephobjectstore", deploymanager.DefaultStorageClusterName)))

				cbps := &cephv1.CephBlockPoolList{}
				cbps, err = deployManager.GetRookCephClient().
						CephBlockPools(namespace).
						List(metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(len(cbps.Items)).To(Equal(1))
				Expect(cbps.Items[0].Name).To(Equal(fmt.Sprintf("%s-cephblockpool", deploymanager.DefaultStorageClusterName)))
				/* cbps := &cephv1.CephBlockPoolList{}
				err = deployManager.GetRookCephClient().
						CephObjectStoresGetter().
						Get().
						Resource("CephBlockPool").
						Namespace(namespace).
						VersionedParams(&metav1.ListOptions{}, parameterCodec).
						Do().
						Into(scis)
				Expect(err).To(BeNil())
				Expect(len(cbps.Items)).To(Equal(1))
				Expect(cbps.Items[0].Name).To(Equal(util.PhaseReady))

				cosus := &cephv1.CephObjectStoreUserList{}
				err = deployManager.GetRookCephClient().Get().
						Resource("CephObjectStoreUser").
						Namespace(namespace).
						VersionedParams(&metav1.ListOptions{}, parameterCodec).
						Do().
						Into(scis)
				Expect(err).To(BeNil())
				Expect(len(cosus.Items)).To(Equal(1))
				Expect(cosus.Items[0].Name).To(Equal(util.PhaseReady))

				cfss := &cephv1.CephFilesystemList{}
				err = deployManager.GetRookCephClient().Get().
						Resource("CephFilesystem").
						Namespace(namespace).
						VersionedParams(&metav1.ListOptions{}, parameterCodec).
						Do().
						Into(scis)
				Expect(err).To(BeNil())
				Expect(len(cfss.Items)).To(Equal(1))
				Expect(cfss.Items[0].Name).To(Equal(util.PhaseReady))

				scs := &storagev1.StorageClassList{}
				err = deployManager.GetRookCephClient().Get().
						Resource("storageclasses").
						Namespace(namespace).
						VersionedParams(&metav1.ListOptions{}, parameterCodec).
						Do().
						Into(scis)
				Expect(err).To(BeNil())
				Expect(len(scis.Items)).To(Equal(3))
				Expect(scs.Items[0].Name).To(Equal(util.PhaseReady)) */
			})
		})
		Context("Delete", func() {
			FIt("And verify it is recreated after delete", func() {
				deployManager, err := deploymanager.NewDeployManager()

				cobs := &cephv1.CephObjectStoreList{}
				err = deployManager.GetRookCephClient().
						CephObjectStores(namespace).
						Delete(fmt.Sprintf("%s-cephobjectstore", deploymanager.DefaultStorageClusterName), &metav1.DeleteOptions{})
				Expect(err).To(BeNil())

				cobs, err = deployManager.GetRookCephClient().
						CephObjectStores(namespace).
						List(metav1.ListOptions{})
				Expect(err).To(BeNil())
				Expect(len(cobs.Items)).To(Equal(0))

				Expect(err).To(BeNil())
				ocsClient = deployManager.GetOcsClient()
				parameterCodec = deployManager.GetParameterCodec()
				scis := &ocsv1.StorageClusterInitializationList{}
				err = ocsClient.Delete().
						Resource("storageclusterinitializations").
						Namespace(namespace).
						VersionedParams(&metav1.DeleteOptions{}, parameterCodec).
						Do().
						Error()
				Expect(err).To(BeNil())

				Eventually(func() string {
					ocsClient = deployManager.GetOcsClient()
					parameterCodec = deployManager.GetParameterCodec()
					err = ocsClient.Get().
							Resource("storageclusterinitializations").
							Namespace(namespace).
							VersionedParams(&metav1.ListOptions{}, parameterCodec).
							Do().
							Into(scis)
					fmt.Println(scis)
					return scis.Items[0].Status.Phase
				}, 30*time.Second, 1*time.Second).Should(Equal(util.PhaseReady))
				
				Eventually(func() string {
					cobs, err = deployManager.GetRookCephClient().
							CephObjectStores(namespace).
							List(metav1.ListOptions{})
					return cobs.Items[0].Name
				}, 30*time.Second, 1*time.Second).Should(Equal(fmt.Sprintf("%s-cephblockstore", deploymanager.DefaultStorageClusterName)))
			})
		})
	})
})
