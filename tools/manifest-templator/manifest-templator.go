package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/ghodss/yaml"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	ocscomponents "github.com/openshift/ocs-operator/pkg/components"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

type operatorData struct {
	Deployment     string
	DeploymentSpec string
	OperatorTag    string
	ComponentTag   string

	// Operator can have multiple CRDS, Roles and ClusterRoles.
	// Store as key-values: objectMeta.Name=>value.
	Roles            map[string]string
	RoleRules        map[string]string
	ClusterRoles     map[string]string
	ClusterRoleRules map[string]string
	CRDs             map[string]*extv1beta1.CustomResourceDefinition
	CRDStrings       map[string]string
	CRStrings        map[string]string
}

type templateData struct {
	Converged       bool
	Namespace       string
	CsvVersion      string
	ReplacesVersion string
	Replaces        bool
	ImagePullPolicy string
	CreatedAt       string
	OCS             *operatorData
	RCO             *operatorData
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func fixResourceString(in string, indention int) string {
	out := strings.Builder{}
	scanner := bufio.NewScanner(strings.NewReader(in))
	for scanner.Scan() {
		line := scanner.Text()
		// remove separator lines
		if !strings.HasPrefix(line, "---") {
			// indent so that it fits into the manifest
			// spaces is indention - 2, because we want to have 2 spaces less for being able to start an array
			spaces := strings.Repeat(" ", indention-2)
			if strings.HasPrefix(line, "apiGroups") {
				// spaces + array start
				out.WriteString(spaces + "- " + line + "\n")
			} else {
				// 2 more spaces
				out.WriteString(spaces + "  " + line + "\n")
			}
		}
	}
	return out.String()
}

func marshallObject(obj interface{}, writer io.Writer) error {
	jsonBytes, err := json.Marshal(obj)
	check(err)

	var r unstructured.Unstructured
	if err := json.Unmarshal(jsonBytes, &r.Object); err != nil {
		return err
	}

	// remove status and metadata.creationTimestamp
	unstructured.RemoveNestedField(r.Object, "template", "metadata", "creationTimestamp")
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

	// fix templates by removing quotes...
	s := string(yamlBytes)
	s = strings.Replace(s, "'{{", "{{", -1)
	s = strings.Replace(s, "}}'", "}}", -1)
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

func getOCS(data *templateData) {
	writer := strings.Builder{}

	// Get OCS Deployment
	ocsdeployment := ocscomponents.GetDeployment(
		"quay.io",
		data.OCS.OperatorTag,
		"Always",
	)
	err := marshallObject(ocsdeployment, &writer)
	check(err)
	deployment := writer.String()

	// Get OCS DeploymentSpec for CSV
	writer = strings.Builder{}
	err = marshallObject(ocsdeployment.Spec, &writer)
	check(err)
	deploymentSpec := fixResourceString(writer.String(), 12)

	// Get OCS Role
	writer = strings.Builder{}
	role := ocscomponents.GetRole()
	marshallObject(role, &writer)
	roleString := writer.String()

	// Get the Rules out of OCS Role
	writer = strings.Builder{}
	ocsrules := role.Rules
	for _, rule := range ocsrules {
		err := marshallObject(rule, &writer)
		check(err)
	}
	rules := fixResourceString(writer.String(), 14)

	data.OCS.Deployment = deployment
	data.OCS.DeploymentSpec = deploymentSpec

	data.OCS.Roles = make(map[string]string)
	data.OCS.RoleRules = make(map[string]string)
	data.OCS.Roles[role.ObjectMeta.Name] = roleString
	data.OCS.RoleRules[role.ObjectMeta.Name] = rules

	data.OCS.CRDs = make(map[string]*extv1beta1.CustomResourceDefinition)
	data.OCS.CRDStrings = make(map[string]string)
	data.OCS.CRStrings = make(map[string]string)

	// Get OCS CRD
	crds := ocscomponents.GetCRDs()
	for _, crd := range crds {
		writer = strings.Builder{}
		marshallObject(crd, &writer)
		crdString := writer.String()

		data.OCS.CRDs[crd.ObjectMeta.Name] = crd
		data.OCS.CRDStrings[crd.ObjectMeta.Name] = crdString
	}

	// Get StorageCluster CR
	writer = strings.Builder{}
	sccr := ocscomponents.GetStorageClusterCR()
	marshallObject(sccr, &writer)
	crString := writer.String()
	data.OCS.CRStrings[sccr.ObjectMeta.Name] = crString

	// Get OCSInitialization CR
	writer = strings.Builder{}
	ocsicr := ocscomponents.GetOCSInitializationCR()
	marshallObject(ocsicr, &writer)
	crString = writer.String()
	data.OCS.CRStrings[ocsicr.ObjectMeta.Name] = crString
}

func getRCO(data *templateData) {
	writer := strings.Builder{}

	// Get RCO Deployment
	ocsdeployment := ocscomponents.GetRookCephDeployment(
		"quay.io",
		data.RCO.OperatorTag,
		"Always",
	)
	err := marshallObject(ocsdeployment, &writer)
	check(err)
	deployment := writer.String()

	// Get RCO DeploymentSpec for CSV
	writer = strings.Builder{}
	err = marshallObject(ocsdeployment.Spec, &writer)
	check(err)
	deploymentSpec := fixResourceString(writer.String(), 12)

	data.RCO.Roles = make(map[string]string)
	data.RCO.RoleRules = make(map[string]string)

	// Get RCO Roles
	roles := ocscomponents.GetRookCephRoles()
	for _, role := range roles {
		writer = strings.Builder{}
		marshallObject(role, &writer)
		roleString := writer.String()
		data.RCO.Roles[role.ObjectMeta.Name] = roleString

		// Get the Rules out of RCO ClusterRole
		writer = strings.Builder{}
		ocsrules := role.Rules
		for _, rule := range ocsrules {
			err := marshallObject(rule, &writer)
			check(err)
		}
		rules := fixResourceString(writer.String(), 14)
		data.RCO.RoleRules[role.ObjectMeta.Name] = rules
	}

	data.RCO.ClusterRoles = make(map[string]string)
	data.RCO.ClusterRoleRules = make(map[string]string)

	// Get RCO ClusterRoles
	clusterRoles := ocscomponents.GetRookCephClusterRoles()
	for _, role := range clusterRoles {
		writer = strings.Builder{}
		marshallObject(role, &writer)
		roleString := writer.String()
		data.RCO.ClusterRoles[role.ObjectMeta.Name] = roleString

		// Get the Rules out of RCO ClusterRole
		writer = strings.Builder{}
		ocsrules := role.Rules
		for _, rule := range ocsrules {
			err := marshallObject(rule, &writer)
			check(err)
		}
		rules := fixResourceString(writer.String(), 14)
		data.RCO.ClusterRoleRules[role.ObjectMeta.Name] = rules
	}

	data.RCO.CRDs = make(map[string]*extv1beta1.CustomResourceDefinition)
	data.RCO.CRDStrings = make(map[string]string)

	// Get RCO CRDs
	crds := ocscomponents.GetRookCephCRDs()
	for _, crd := range crds {
		writer = strings.Builder{}
		marshallObject(crd, &writer)
		crdString := writer.String()

		data.RCO.CRDs[crd.ObjectMeta.Name] = crd
		data.RCO.CRDStrings[crd.ObjectMeta.Name] = crdString
	}

	data.RCO.Deployment = deployment
	data.RCO.DeploymentSpec = deploymentSpec
}

func main() {
	converged := flag.Bool("converged", false, "")
	namespace := flag.String("namespace", "storageclusters", "")
	csvVersion := flag.String("csv-version", "0.0.2", "")
	replacesVersion := flag.String("replaces-version", "0.0.1", "")
	imagePullPolicy := flag.String("image-pull-policy", "IfNotPresent", "")
	inputFile := flag.String("input-file", "", "")

	containerTag := flag.String("container-tag", "latest", "")
	ocsTag := flag.String("ocs-tag", *containerTag, "")
	rcoTag := flag.String("rco-tag", *containerTag, "")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.CommandLine.ParseErrorsWhitelist.UnknownFlags = true
	pflag.Parse()

	Replaces := true
	if *replacesVersion == *csvVersion {
		Replaces = false
	}

	data := templateData{
		Converged:       *converged,
		Namespace:       *namespace,
		CsvVersion:      *csvVersion,
		ReplacesVersion: *replacesVersion,
		Replaces:        Replaces,
		ImagePullPolicy: *imagePullPolicy,

		OCS: &operatorData{OperatorTag: *ocsTag, ComponentTag: *ocsTag},
		RCO: &operatorData{OperatorTag: *rcoTag, ComponentTag: *rcoTag},
	}
	data.CreatedAt = time.Now().String()

	// Load in all OCS Resources
	getOCS(&data)

	// Load in all RCO Resources
	getRCO(&data)

	if *inputFile == "" {
		panic("Must specify input file")
	}

	manifestTemplate := template.Must(template.ParseFiles(*inputFile))
	err := manifestTemplate.Execute(os.Stdout, data)
	check(err)
}
