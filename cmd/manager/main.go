package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	nbapis "github.com/noobaa/noobaa-operator/v2/pkg/apis"
	"github.com/openshift/ocs-operator/pkg/apis"
	ocsv1 "github.com/openshift/ocs-operator/pkg/apis/ocs/v1"
	"github.com/openshift/ocs-operator/pkg/controller"
	"github.com/openshift/ocs-operator/pkg/controller/ocsinitialization"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/ready"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	cephv1 "github.com/rook/rook/pkg/apis/ceph.rook.io/v1"
)

var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("operator-sdk Version: %v", sdkVersion.Version))
}

func main() {
	flag.Parse()

	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(logf.ZapLogger(false))

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "failed to get watch namespace")
		os.Exit(1)
	}

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Become the leader before proceeding
	leader.Become(context.TODO(), "ocs-operator-lock")

	r := ready.NewFileReady()
	err = r.Set()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
	defer r.Unset()

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{Namespace: namespace})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all api resources
	mgrScheme := mgr.GetScheme()

	if err := apis.AddToScheme(mgrScheme); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err := cephv1.AddToScheme(mgrScheme); err != nil {
		log.Error(err, "Failed adding cephv1 to scheme")
		os.Exit(1)
	}

	if err := storagev1.AddToScheme(mgrScheme); err != nil {
		log.Error(err, "Failed adding storage/v1 to scheme")
		os.Exit(1)
	}

	if err := nbapis.AddToScheme(mgrScheme); err != nil {
		log.Error(err, "Failed adding noobaa apis to scheme")
		os.Exit(1)
	}

	if err := monitoringv1.AddToScheme(mgrScheme); err != nil {
		log.Error(err, "Failed adding monitoring/v1 apis to scheme")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create CR if it's not there
	ocsNamespacedName := ocsinitialization.InitNamespacedName()
	client := mgr.GetClient()
	err = client.Create(context.TODO(), &ocsv1.OCSInitialization{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ocsNamespacedName.Name,
			Namespace: ocsNamespacedName.Namespace,
		},
	})
	switch {
	case err == nil:
		log.Info("Created OCSInitialization resource")
	case errors.IsAlreadyExists(err):
		log.Info("OCSInitialization resource already exists")
	default:
		log.Error(err, "Failed to create OCSInitialization custom resource")
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "manager exited non-zero")
		os.Exit(1)
	}
}
