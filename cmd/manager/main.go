/*
Copyright 2019 The Kubernetes Authors.

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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	bmoapis "github.com/metalkube/baremetal-operator/pkg/apis"
	"github.com/metalkube/cluster-api-provider-baremetal/pkg/apis"
	"github.com/metalkube/cluster-api-provider-baremetal/pkg/cloud/baremetal/actuators/machine"
	clusterapis "github.com/openshift/cluster-api/pkg/apis"
	capimachine "github.com/openshift/cluster-api/pkg/controller/machine"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	klog.InitFlags(nil)

	metricsAddr := flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.Parse()

	log := logf.Log.WithName("baremetal-controller-manager")
	logf.SetLogger(logf.ZapLogger(false))
	entryLog := log.WithName("entrypoint")

	cfg := config.GetConfigOrDie()
	if cfg == nil {
		panic(fmt.Errorf("GetConfigOrDie didn't die"))
	}

	err := waitForAPIs(cfg)
	if err != nil {
		entryLog.Error(err, "unable to discover required APIs")
		os.Exit(1)
	}

	// Setup a Manager
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: *metricsAddr})
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	machineActuator, err := machine.NewActuator(machine.ActuatorParams{
		Client: mgr.GetClient(),
	})
	if err != nil {
		panic(err)
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	if err := clusterapis.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	if err := bmoapis.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	capimachine.AddWithActuator(mgr, machineActuator)

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

func waitForAPIs(cfg *rest.Config) error {
	log := logf.Log.WithName("baremetal-controller-manager")

	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}

	metalkubeGV := schema.GroupVersion{
		Group:   "metalkube.org",
		Version: "v1alpha1",
	}

	for {
		err = discovery.ServerSupportsVersion(c, metalkubeGV)
		if err != nil {
			log.Info(fmt.Sprintf("Waiting for API group %v to be available: %v", metalkubeGV, err))
			time.Sleep(time.Second * 10)
			continue
		}
		log.Info(fmt.Sprintf("Found API group %v", metalkubeGV))
		break
	}

	return nil
}
