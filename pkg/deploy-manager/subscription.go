package deploymanager

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	yaml "github.com/ghodss/yaml"
	"github.com/operator-framework/operator-lifecycle-manager/pkg/controller/install"
	v1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	v1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

const localStorageNamespace = "local-storage"
const marketplaceNamespace = "openshift-marketplace"
const defaultLocalStorageRegistryImage = "quay.io/gnufied/local-registry:v4.2.0"
const defaultOcsRegistryImage = "quay.io/ocs-dev/ocs-registry:latest"

type clusterObjects struct {
	namespaces     []k8sv1.Namespace
	operatorGroups []v1.OperatorGroup
	catalogSources []v1alpha1.CatalogSource
	subscriptions  []v1alpha1.Subscription
}

func (t *DeployManager) deployClusterObjects(co *clusterObjects) error {

	for _, namespace := range co.namespaces {
		err := t.CreateNamespace(namespace.Name)
		if err != nil {
			return err
		}
	}

	for _, operatorGroup := range co.operatorGroups {
		_, err := t.olmClient.OperatorsV1().OperatorGroups(operatorGroup.Namespace).Create(&operatorGroup)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}

	}

	for _, catalogSource := range co.catalogSources {
		_, err := t.olmClient.OperatorsV1alpha1().CatalogSources(catalogSource.Namespace).Create(&catalogSource)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// Wait for catalog source before posting subscription
	err := t.waitForOCSCatalogSource()
	if err != nil {
		return err
	}

	for _, subscription := range co.subscriptions {
		_, err := t.olmClient.OperatorsV1alpha1().Subscriptions(subscription.Namespace).Create(&subscription)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// Wait on ocs-operator, rook-ceph-operator and noobaa-operator to come online.
	err = t.WaitForOCSOperator()
	if err != nil {
		return err
	}

	return nil
}

func (t *DeployManager) deleteClusterObjects(co *clusterObjects) error {

	for _, operatorGroup := range co.operatorGroups {
		err := t.olmClient.OperatorsV1().OperatorGroups(operatorGroup.Namespace).Delete(operatorGroup.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
		    return err
		}

	}

	for _, catalogSource := range co.catalogSources {
		err := t.olmClient.OperatorsV1alpha1().CatalogSources(catalogSource.Namespace).Delete(catalogSource.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
		    return err
		}
	}

	for _, subscription := range co.subscriptions {
		err := t.olmClient.OperatorsV1alpha1().Subscriptions(subscription.Namespace).Delete(subscription.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
		    return err
		}
	}

	for _, namespace := range co.namespaces {
		err := t.DeleteNamespaceAndWait(namespace.Name)
		if err != nil {
		    return err
		}
	}

	return nil
}

func (t *DeployManager) generateClusterObjects(ocsRegistryImage string, localStorageRegistryImage string, subscriptionChannel string) *clusterObjects {

	co := &clusterObjects{}
	label := make(map[string]string)
	// Label required for monitoring this namespace
	label["openshift.io/cluster-monitoring"] = "true"

	// Namespaces
	co.namespaces = append(co.namespaces, k8sv1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   InstallNamespace,
			Labels: label,
		},
	})
	co.namespaces = append(co.namespaces, k8sv1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: localStorageNamespace,
		},
	})

	// Operator Groups
	ocsOG := v1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openshift-storage-operatorgroup",
			Namespace: InstallNamespace,
		},
		Spec: v1.OperatorGroupSpec{
			TargetNamespaces: []string{InstallNamespace},
		},
	}
	ocsOG.SetGroupVersionKind(schema.GroupVersionKind{Group: v1.SchemeGroupVersion.Group, Kind: "OperatorGroup", Version: v1.SchemeGroupVersion.Version})

	localStorageOG := v1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-operator-group",
			Namespace: localStorageNamespace,
		},
		Spec: v1.OperatorGroupSpec{
			TargetNamespaces: []string{InstallNamespace},
		},
	}
	localStorageOG.SetGroupVersionKind(schema.GroupVersionKind{Group: v1.SchemeGroupVersion.Group, Kind: "OperatorGroup", Version: v1.SchemeGroupVersion.Version})

	co.operatorGroups = append(co.operatorGroups, ocsOG)
	co.operatorGroups = append(co.operatorGroups, localStorageOG)

	// CatalogSources
	localStorageCatalog := v1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-storage-manifests",
			Namespace: marketplaceNamespace,
		},
		Spec: v1alpha1.CatalogSourceSpec{
			SourceType:  v1alpha1.SourceTypeGrpc,
			Image:       localStorageRegistryImage,
			DisplayName: "Local Storage Operator",
			Publisher:   "Red Hat",
			Description: "An operator to manage local volumes",
		},
	}
	localStorageCatalog.SetGroupVersionKind(schema.GroupVersionKind{Group: v1alpha1.SchemeGroupVersion.Group, Kind: "CatalogSource", Version: v1alpha1.SchemeGroupVersion.Version})

	ocsCatalog := v1alpha1.CatalogSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ocs-catalogsource",
			Namespace: marketplaceNamespace,
		},
		Spec: v1alpha1.CatalogSourceSpec{
			SourceType:  v1alpha1.SourceTypeGrpc,
			Image:       ocsRegistryImage,
			DisplayName: "Openshift Container Storage",
			Publisher:   "Red Hat",
		},
	}
	ocsCatalog.SetGroupVersionKind(schema.GroupVersionKind{Group: v1alpha1.SchemeGroupVersion.Group, Kind: "CatalogSource", Version: v1alpha1.SchemeGroupVersion.Version})

	co.catalogSources = append(co.catalogSources, localStorageCatalog)
	co.catalogSources = append(co.catalogSources, ocsCatalog)

	// Subscriptions
	ocsSubscription := v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ocs-subscription",
			Namespace: InstallNamespace,
		},
		Spec: &v1alpha1.SubscriptionSpec{
			Channel:                subscriptionChannel,
			Package:                "ocs-operator",
			CatalogSource:          "ocs-catalogsource",
			CatalogSourceNamespace: marketplaceNamespace,
		},
	}
	ocsSubscription.SetGroupVersionKind(schema.GroupVersionKind{Group: v1alpha1.SchemeGroupVersion.Group, Kind: "Subscription", Version: v1alpha1.SchemeGroupVersion.Version})

	co.subscriptions = append(co.subscriptions, ocsSubscription)

	return co
}

func marshallObject(obj interface{}, writer io.Writer) error {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		return err
	}

	unstructured.RemoveNestedField(r.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(r.Object, "status")

	jsonBytes, err = json.Marshal(r.Object)
	if err != nil {
		return err
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return err
	}

	// fix double quoted strings by removing unneeded single quotes...
	s := string(yamlBytes)
	s = strings.Replace(s, " '\"", " \"", -1)
	s = strings.Replace(s, "\"'\n", "\"\n", -1)

	yamlBytes = []byte(s)

	_, err = writer.Write([]byte("---\n"))
	if err != nil {
		return err
	}

	_, err = writer.Write(yamlBytes)
	if err != nil {
		return err
	}

	return nil
}

// DumpYAML dumps ocs deployment yaml
func (t *DeployManager) DumpYAML(ocsRegistryImage string, localStorageRegistryImage string, subscriptionChannel string) string {
	co := t.generateClusterObjects(ocsRegistryImage, localStorageRegistryImage, subscriptionChannel)

	writer := strings.Builder{}

	for _, namespace := range co.namespaces {
		marshallObject(namespace, &writer)
	}

	for _, operatorGroup := range co.operatorGroups {
		marshallObject(operatorGroup, &writer)
	}

	for _, catalogSource := range co.catalogSources {
		marshallObject(catalogSource, &writer)
	}

	for _, subscription := range co.subscriptions {
		marshallObject(subscription, &writer)
	}

	return writer.String()
}

func (t *DeployManager) waitForOCSCatalogSource() error {
	timeout := 300 * time.Second
	interval := 10 * time.Second

	lastReason := ""

	labelSelector, err := labels.Parse("olm.catalogSource in (ocs-catalogsource)")
	if err != nil {
		return err
	}

	err = utilwait.PollImmediate(interval, timeout, func() (done bool, err error) {
		pods, err := t.k8sClient.CoreV1().Pods(marketplaceNamespace).List(metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		})
		if err != nil {
			lastReason = fmt.Sprintf("error talking to k8s apiserver: %v", err)
			return false, nil
		}

		if len(pods.Items) == 0 {
			lastReason = "waiting on ocs catalog source pod to be created"
			return false, nil
		}
		isReady := false
		for _, pod := range pods.Items {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == k8sv1.PodReady && condition.Status == k8sv1.ConditionTrue {
					isReady = true
					break
				}
			}
		}

		if !isReady {
			lastReason = "waiting on ocs catalog source pod to reach ready state"
			return false, nil
		}

		// if we get here, then all deployments are created and available
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("%v: %s", err, lastReason)
	}

	return nil
}

// DeployOCSWithOLM deploys ocs operator via an olm subscription
func (t *DeployManager) DeployOCSWithOLM(ocsRegistryImage string, localStorageRegistryImage string, subscriptionChannel string) error {

	if ocsRegistryImage == "" || localStorageRegistryImage == "" {
		return fmt.Errorf("catalog registry images not supplied")
	}

	co := t.generateClusterObjects(ocsRegistryImage, localStorageRegistryImage, subscriptionChannel)
	err := t.deployClusterObjects(co)
	if err != nil {
		return err
	}

	return nil
}

// UpgradeOCSWithOLM upgrades ocs operator via an olm subscription
func (t *DeployManager) UpgradeOCSWithOLM(ocsRegistryImage string, localStorageRegistryImage string, subscriptionChannel string) error {

	if ocsRegistryImage == "" || localStorageRegistryImage == "" {
		return fmt.Errorf("catalog registry images not supplied")
	}

	co := t.generateClusterObjects(ocsRegistryImage, localStorageRegistryImage, subscriptionChannel)
	err := t.updateClusterObjects(co)
	if err != nil {
		return err
	}

	return nil
}

// UninstallOCS uninstalls ocs operator and storage clusters
func (t *DeployManager) UninstallOCS(ocsRegistryImage string, localStorageRegistryImage string, subscriptionChannel string) error {
	// Remove finalizers from all cephclusters to not block the cleanup
	err := t.removeCephClusterFinalizers()
	if err != nil {
		return err
	}

	// delete storage clusters and storareclusterinitialization objects
	err = t.deleteStorageClusters()
	if err != nil {
		return err
	}

	err = t.deleteNoobaaSystems()
	if err != nil {
		//return err
	}

	err = t.k8sClient.AppsV1().StatefulSets(InstallNamespace).Delete("noobaa-core", &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Delete all noobaa-core related pods
	err = t.k8sClient.CoreV1().Pods(InstallNamespace).DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "noobaa-core"})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	err = t.deleteCephClusters()
	if err != nil {
		//return err
	}

	// Delete all operator deployments
	deployments := []string{"noobaa-operator", "rook-ceph-operator", "ocs-operator"}
	for _, name := range deployments {
		err := t.k8sClient.AppsV1().Deployments(InstallNamespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete all subscriptions in the namespace
	subscriptions, err := t.olmClient.OperatorsV1alpha1().Subscriptions(InstallNamespace).List(metav1.ListOptions{})
	for _, subscription := range subscriptions.Items {
		err := t.olmClient.OperatorsV1alpha1().Subscriptions(InstallNamespace).Delete(subscription.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete all remaining deployments in the namespace
	deployments1, err := t.k8sClient.AppsV1().Deployments(InstallNamespace).List(metav1.ListOptions{})
	for _, deployment := range deployments1.Items {
		err := t.k8sClient.AppsV1().Deployments(InstallNamespace).Delete(deployment.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete all remaining daemonsets in the namespace
	daemonsets, err := t.k8sClient.AppsV1().DaemonSets(InstallNamespace).List(metav1.ListOptions{})
	for _, daemonset := range daemonsets.Items {
		err := t.k8sClient.AppsV1().DaemonSets(InstallNamespace).Delete(daemonset.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete all remaining pods in the namespace
	pods, err := t.k8sClient.CoreV1().Pods(InstallNamespace).List(metav1.ListOptions{})
	for _, pod := range pods.Items {
		err := t.k8sClient.CoreV1().Pods(InstallNamespace).Delete(pod.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete all PVCs in the namespace
	pvcs, err := t.k8sClient.CoreV1().PersistentVolumeClaims(InstallNamespace).List(metav1.ListOptions{})
	for _, pvc := range pvcs.Items {
		err := t.k8sClient.CoreV1().PersistentVolumeClaims(InstallNamespace).Delete(pvc.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete all PVs in the namespace
	pvs, err := t.k8sClient.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
	for _, pv := range pvs.Items {
		err := t.k8sClient.CoreV1().PersistentVolumes().Delete(pv.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	// Delete remaining operator manifests
	co := t.generateClusterObjects(ocsRegistryImage, localStorageRegistryImage, subscriptionChannel)
	err = t.deleteClusterObjects(co)
	if err != nil {
		return err
	}

	return nil
}

// WaitForOCSOperator waits for the ocs-operator to come online
func (t *DeployManager) WaitForOCSOperator() error {
	deployments := []string{"ocs-operator", "rook-ceph-operator", "noobaa-operator"}

	timeout := 1000 * time.Second
	interval := 10 * time.Second

	lastReason := ""

	err := utilwait.PollImmediate(interval, timeout, func() (done bool, err error) {
		for _, name := range deployments {
			deployment, err := t.k8sClient.AppsV1().Deployments(InstallNamespace).Get(name, metav1.GetOptions{})
			if err != nil {
				lastReason = fmt.Sprintf("waiting on deployment %s to be created", name)
				return false, nil
			}

			isAvailable := false
			for _, condition := range deployment.Status.Conditions {
				if condition.Type == appsv1.DeploymentAvailable && condition.Status == k8sv1.ConditionTrue {
					isAvailable = true
					break
				}
			}

			if !isAvailable {
				lastReason = fmt.Sprintf("waiting on deployment %s to become available", name)
				return false, nil
			}
		}

		// if we get here, then all deployments are created and available
		return true, nil
	})

	if err != nil {
		return fmt.Errorf("%v: %s", err, lastReason)
	}

	return nil
}

func (t *DeployManager) updateClusterObjects(co *clusterObjects) error {
	for _, catalogSource := range co.catalogSources {
		cs, err := t.olmClient.OperatorsV1alpha1().CatalogSources(catalogSource.Namespace).Get(catalogSource.Name, metav1.GetOptions{})
		cs.Spec.Image = catalogSource.Spec.Image
		_, err = t.olmClient.OperatorsV1alpha1().CatalogSources(catalogSource.Namespace).Update(cs)
		if err != nil {
			return err
		}

	}

	// TODO: Verify this is a new catalog source. But does it have to be a new catalogsource?
	// Can we upgrade to a new subscription channel?
	// Wait for catalog source before updating subscription
	err := t.waitForOCSCatalogSource()
	if err != nil {
		return err
	}

	for _, subscription := range co.subscriptions {
		sub, err := t.olmClient.OperatorsV1alpha1().Subscriptions(subscription.Namespace).Get(subscription.Name, metav1.GetOptions{})
		sub.Spec.Channel = subscription.Spec.Channel
		_, err = t.olmClient.OperatorsV1alpha1().Subscriptions(subscription.Namespace).Update(sub)
		if err != nil {
			return err
		}

	}
	return nil
}

// WaitForCsvUpgrade waits for the catalogsource to come online after an upgrade
func (t *DeployManager) WaitForCsvUpgrade(csvName string, subscriptionChannel string) error {
	timeout := 1200 * time.Second
	// NOTE the long timeout above. It can take quite a bit of time for the
	// ocs operator deployments to roll out
	interval := 10 * time.Second

	subscription := "ocs-subscription"
	operatorName := "ocs-operator"

	lastReason := ""
	waitErr := utilwait.PollImmediate(interval, timeout, func() (done bool, err error) {
		sub, err := t.olmClient.OperatorsV1alpha1().Subscriptions(InstallNamespace).Get(subscription, metav1.GetOptions{})
		if sub.Spec.Channel != subscriptionChannel {
			lastReason = fmt.Sprintf("waiting on subscription channel to be updated to %s ", subscriptionChannel)
			return false, nil
		}
		csvs, err := t.olmClient.OperatorsV1alpha1().ClusterServiceVersions(InstallNamespace).List(metav1.ListOptions{})
		for _, csv := range csvs.Items {
			// If the csvName doesn't match, it means a new csv has appeared
			if csv.Name != csvName && strings.Contains(csv.Name, operatorName) {
				// New csv found and phase is succeeeded
				if csv.Status.Phase == "Succeeded" {
					return true, nil
				}
			}
		}
		lastReason = fmt.Sprintf("waiting on csv to be created and installed")
		return false, nil
	})

	if waitErr != nil {
		return fmt.Errorf("%v: %s", waitErr, lastReason)
	}

	return nil
}

// GetCsv retrieves the csv named ocs-operator
func (t *DeployManager) GetCsv() (v1alpha1.ClusterServiceVersion, error){
	csvName := "ocs-operator"
	csv := v1alpha1.ClusterServiceVersion{}
	csvs, err := t.olmClient.OperatorsV1alpha1().ClusterServiceVersions(InstallNamespace).List(metav1.ListOptions{})
	for _, csv := range csvs.Items {
		if strings.Contains(csv.Name, csvName) {
			return csv, err
		}
	}
	return csv, err
}

// VerifyComponentOperators makes sure that deployment images matches the ones specified in the csv deployment specs
func (t *DeployManager) VerifyComponentOperators() error {
	csv, err := t.GetCsv()
	if err != nil {
		return err
	}

	resolver := install.StrategyResolver{}
	strategy, err := resolver.UnmarshalStrategy(csv.Spec.InstallStrategy)
	if err != nil {
		return err
	}

	strategyDetailsDeployment, _ := strategy.(*install.StrategyDetailsDeployment)
	for _, deployment := range strategyDetailsDeployment.DeploymentSpecs {
		image := deployment.Spec.Template.Spec.Containers[0].Image
		foundImage, err := t.GetDeploymentImage(deployment.Name)
		if err != nil {
			return err
		}
		if image != foundImage {
			return fmt.Errorf("Deployment: %s Expected image: %s Found image  %s", deployment.Name, image, foundImage)
		}
	}
	return nil
}
