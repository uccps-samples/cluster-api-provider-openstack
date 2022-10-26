/*
Copyright 2018 The Kubernetes Authors.

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

package machine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/record"

	machinev1 "github.com/uccps-samples/api/machine/v1beta1"
	apierrors "github.com/uccps-samples/machine-api-operator/pkg/controller/machine"
	"github.com/uccps-samples/machine-api-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	tokenapi "k8s.io/cluster-bootstrap/token/api"
	tokenutil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/klog/v2"
	openstackconfigv1 "sigs.k8s.io/cluster-api-provider-openstack/pkg/apis/openstackproviderconfig/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-openstack/pkg/bootstrap"
	"sigs.k8s.io/cluster-api-provider-openstack/pkg/cloud/openstack"
	"sigs.k8s.io/cluster-api-provider-openstack/pkg/cloud/openstack/clients"
	"sigs.k8s.io/cluster-api-provider-openstack/pkg/cloud/openstack/options"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clconfig "github.com/coreos/container-linux-config-transpiler/config"
)

const (
	CloudConfigPath = "/etc/cloud/cloud_config.yaml"

	UserDataKey          = "userData"
	DisableTemplatingKey = "disableTemplating"
	PostprocessorKey     = "postprocessor"

	TimeoutInstanceCreate       = 5
	TimeoutInstanceDelete       = 5
	RetryIntervalInstanceStatus = 10 * time.Second

	// MachineInstanceStateAnnotationName as annotation name for a machine instance state
	MachineInstanceStateAnnotationName = "machine.uccp.io/instance-state"

	// ErrorState is assigned to the machine if its instance has been destroyed
	ErrorState = "ERROR"

	OpenstackIdAnnotationKey = "openstack-resourceId"
)

// Event Action Constants
const (
	createEventAction = "Create"
	updateEventAction = "Update"
	deleteEventAction = "Delete"
	noEventAction     = ""
)

type OpenstackClient struct {
	params        openstack.ActuatorParams
	scheme        *runtime.Scheme
	client        client.Client
	eventRecorder record.EventRecorder
}

func NewActuator(params openstack.ActuatorParams) (*OpenstackClient, error) {
	return &OpenstackClient{
		params:        params,
		client:        params.Client,
		scheme:        params.Scheme,
		eventRecorder: params.EventRecorder,
	}, nil
}

func getTimeout(name string, timeout int) time.Duration {
	if v := os.Getenv(name); v != "" {
		timeout, err := strconv.Atoi(v)
		if err == nil {
			return time.Duration(timeout)
		}
	}
	return time.Duration(timeout)
}

func (oc *OpenstackClient) getClusterInfraName() (string, error) {
	clusterInfra, err := oc.params.ConfigClient.Infrastructures().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve cluster Infrastructure object: %v", err)
	}

	return clusterInfra.Status.InfrastructureName, nil
}

func (oc *OpenstackClient) Create(ctx context.Context, machine *machinev1.Machine) error {
	// First check that provided labels are correct
	// TODO(mfedosin): stop sending the infrastructure request when we start to receive the cluster value
	clusterInfraName, err := oc.getClusterInfraName()
	if err != nil {
		return err
	}

	clusterNameLabel := machine.Labels["machine.uccp.io/cluster-api-cluster"]

	if clusterNameLabel != clusterInfraName {
		klog.Errorf("machine.uccp.io/cluster-api-cluster label value is incorrect: %v, machine %v cannot join cluster %v", clusterNameLabel, machine.ObjectMeta.Name, clusterInfraName)
		verr := apierrors.InvalidMachineConfiguration("machine.uccp.io/cluster-api-cluster label value is incorrect: %v, machine %v cannot join cluster %v", clusterNameLabel, machine.ObjectMeta.Name, clusterInfraName)

		return oc.handleMachineError(machine, verr, createEventAction)
	}

	kubeClient := oc.params.KubeClient

	machineService, err := clients.NewInstanceServiceFromMachine(kubeClient, machine)
	if err != nil {
		return err
	}

	providerSpec, err := openstackconfigv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return oc.handleMachineError(machine, apierrors.InvalidMachineConfiguration(
			"Cannot unmarshal providerSpec field: %v", err), createEventAction)
	}

	if err = oc.validateMachine(machine); err != nil {
		verr := apierrors.InvalidMachineConfiguration("Machine validation failed: %v", err)
		return oc.handleMachineError(machine, verr, createEventAction)
	}

	// Here we check whether we want to create a new instance or recreate the destroyed
	// one. If this is the second case, we have to return an error, because if we just
	// create an instance with the old name, because the CSR for it will not be approved
	// automatically.
	// See https://bugzilla.redhat.com/show_bug.cgi?id=1746369
	if machine.Spec.ProviderID != nil {
		klog.Errorf("The instance has been destroyed for the machine %v, cannot recreate it.\n", machine.ObjectMeta.Name)
		verr := apierrors.InvalidMachineConfiguration("the instance has been destroyed for the machine %v, cannot recreate it.\n", machine.ObjectMeta.Name)

		return oc.handleMachineError(machine, verr, createEventAction)
	}

	// get machine startup script
	var ok bool
	var disableTemplating bool
	var postprocessor string
	var postprocess bool

	userData := []byte{}
	if providerSpec.UserDataSecret != nil {
		namespace := providerSpec.UserDataSecret.Namespace
		if namespace == "" {
			namespace = machine.Namespace
		}

		if providerSpec.UserDataSecret.Name == "" {
			return fmt.Errorf("UserDataSecret name must be provided")
		}

		userDataSecret, err := kubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), providerSpec.UserDataSecret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		userData, ok = userDataSecret.Data[UserDataKey]
		if !ok {
			return fmt.Errorf("Machine's userdata secret %v in namespace %v did not contain key %v", providerSpec.UserDataSecret.Name, namespace, UserDataKey)
		}

		_, disableTemplating = userDataSecret.Data[DisableTemplatingKey]

		var p []byte
		p, postprocess = userDataSecret.Data[PostprocessorKey]

		postprocessor = string(p)
	}

	var userDataRendered string
	if len(userData) > 0 && !disableTemplating {
		// FIXME(mandre) Find the right way to check if machine is part of the control plane
		if machine.ObjectMeta.Name != "" {
			userDataRendered, err = masterStartupScript(machine, string(userData))
			if err != nil {
				return oc.handleMachineError(machine, apierrors.CreateMachine(
					"error creating Openstack instance: %v", err), createEventAction)
			}
		} else {
			klog.Info("Creating bootstrap token")
			token, err := oc.createBootstrapToken()
			if err != nil {
				return oc.handleMachineError(machine, apierrors.CreateMachine(
					"error creating Openstack instance: %v", err), createEventAction)
			}
			userDataRendered, err = nodeStartupScript(machine, token, string(userData))
			if err != nil {
				return oc.handleMachineError(machine, apierrors.CreateMachine(
					"error creating Openstack instance: %v", err), createEventAction)
			}
		}
	} else {
		userDataRendered = string(userData)
	}

	//Read the cluster name from the `machine`.
	clusterName := fmt.Sprintf("%s-%s", machine.Namespace, machine.Labels["machine.uccp.io/cluster-api-cluster"])

	// TODO(egarcia): if we ever use the cluster object, this will benifit from reading from it
	var clusterSpec openstackconfigv1.OpenstackClusterProviderSpec

	if postprocess {
		switch postprocessor {
		// Postprocess with the Container Linux ct transpiler.
		case "ct":
			clcfg, ast, report := clconfig.Parse([]byte(userDataRendered))
			if len(report.Entries) > 0 {
				return fmt.Errorf("Postprocessor error: %s", report.String())
			}

			ignCfg, report := clconfig.Convert(clcfg, "openstack-metadata", ast)
			if len(report.Entries) > 0 {
				return fmt.Errorf("Postprocessor error: %s", report.String())
			}

			ud, err := json.Marshal(&ignCfg)
			if err != nil {
				return fmt.Errorf("Postprocessor error: %s", err)
			}

			userDataRendered = string(ud)

		default:
			return fmt.Errorf("Postprocessor error: unknown postprocessor: '%s'", postprocessor)
		}
	}

	instance, err := machineService.InstanceCreate(clusterName, machine.Name, &clusterSpec, providerSpec, userDataRendered, providerSpec.KeyName, oc.params.ConfigClient)

	if err != nil {
		return oc.handleMachineError(machine, apierrors.CreateMachine(
			"error creating Openstack instance: %v", err), createEventAction)
	}
	instanceCreateTimeout := getTimeout("CLUSTER_API_OPENSTACK_INSTANCE_CREATE_TIMEOUT", TimeoutInstanceCreate)
	instanceCreateTimeout = instanceCreateTimeout * time.Minute
	err = util.PollImmediate(RetryIntervalInstanceStatus, instanceCreateTimeout, func() (bool, error) {
		instance, err = machineService.GetInstance(instance.ID)
		if err != nil {
			return false, nil
		}
		return instance.Status == "ACTIVE", nil
	})
	if err != nil {
		return oc.handleMachineError(machine, apierrors.CreateMachine(
			"error creating Openstack instance: %v", err), createEventAction)
	}

	if providerSpec.FloatingIP != "" {
		err := machineService.AssociateFloatingIP(instance.ID, providerSpec.FloatingIP)
		if err != nil {
			return oc.handleMachineError(machine, apierrors.CreateMachine(
				"Associate floatingIP err: %v", err), createEventAction)
		}

	}

	err = machineService.SetMachineLabels(machine, instance.ID)
	if err != nil {
		return nil
	}

	oc.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Created", "Created machine %v", machine.Name)
	return oc.updateAnnotation(machine, instance, clusterInfraName)
}

func (oc *OpenstackClient) Delete(ctx context.Context, machine *machinev1.Machine) error {
	machineService, err := clients.NewInstanceServiceFromMachine(oc.params.KubeClient, machine)
	if err != nil {
		return err
	}

	instance, err := oc.instanceExists(machine)
	if err != nil {
		return err
	}

	if instance == nil {
		klog.Infof("Skipped deleting %s that is already deleted.\n", machine.Name)
		return nil
	}

	id := machine.ObjectMeta.Annotations[OpenstackIdAnnotationKey]
	err = machineService.InstanceDelete(id)
	if err != nil {
		return oc.handleMachineError(machine, apierrors.DeleteMachine(
			"error deleting Openstack instance: %v", err), deleteEventAction)
	}

	oc.eventRecorder.Eventf(machine, corev1.EventTypeNormal, "Deleted", "Deleted machine %v", machine.Name)
	return nil
}

func (oc *OpenstackClient) Update(ctx context.Context, machine *machinev1.Machine) error {
	clusterInfraName, err := oc.getClusterInfraName()
	if err != nil {
		return err
	}
	instance, err := oc.instanceExists(machine)
	if err != nil {
		return fmt.Errorf("error fetching OpenStack server for machine %s: %w", machine.Name, err)
	}

	return oc.updateAnnotation(machine, instance, clusterInfraName)
}

func (oc *OpenstackClient) Exists(ctx context.Context, machine *machinev1.Machine) (bool, error) {
	instance, err := oc.instanceExists(machine)
	if err != nil {
		return false, fmt.Errorf("Error checking if instance exists (machine/actuator.go 346): %v", err)
	}
	return instance != nil, err
}

func getIPsFromInstance(instance *clients.Instance) ([]corev1.NodeAddress, error) {
	type networkInterface struct {
		Address string  `json:"addr"`
		Version float64 `json:"version"`
		Type    string  `json:"OS-EXT-IPS:type"`
	}

	var nodeAddresses []corev1.NodeAddress

	// This is heavily based on the related upstream code:
	// https://github.com/kubernetes-sigs/cluster-api-provider-openstack/blob/244d31b1d583ee9e760d2bc2f18a80e1fc61f5eb/pkg/cloud/services/compute/instance_types.go#L131-L183
	for _, b := range instance.Addresses {
		list, err := json.Marshal(b)
		if err != nil {
			return nil, fmt.Errorf("error marshalling addresses for instance %s: %w", instance.ID, err)
		}
		var interfaceList []networkInterface
		err = json.Unmarshal(list, &interfaceList)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling addresses for instance %s: %w", instance.ID, err)
		}

		for i := range interfaceList {
			address := &interfaceList[i]

			// Only consider IPv4
			if address.Version != 4 {
				klog.V(6).Info("Ignoring IPv%d address %s: only IPv4 is supported", address.Version, address.Address)
				continue
			}

			var addressType corev1.NodeAddressType
			switch address.Type {
			case "floating":
				addressType = corev1.NodeExternalIP
			case "fixed":
				addressType = corev1.NodeInternalIP
			default:
				klog.V(6).Info("Ignoring address %s with unknown type '%s'", address.Address, address.Type)
				continue
			}

			nodeAddresses = append(nodeAddresses, corev1.NodeAddress{
				Type:    addressType,
				Address: address.Address,
			})
		}
	}

	return nodeAddresses, nil
}

// If the OpenstackClient has a client for updating Machine objects, this will set
// the appropriate reason/message on the Machine.Status. If not, such as during
// cluster installation, it will operate as a no-op. It also returns the
// original error for convenience, so callers can do "return handleMachineError(...)".
func (oc *OpenstackClient) handleMachineError(machine *machinev1.Machine, err *apierrors.MachineError, eventAction string) error {
	if eventAction != noEventAction {
		oc.eventRecorder.Eventf(machine, corev1.EventTypeWarning, "Failed"+eventAction, "%v", err.Reason)
	}
	if oc.client != nil {
		reason := err.Reason
		message := err.Message
		machine.Status.ErrorReason = &reason
		machine.Status.ErrorMessage = &message

		// Set state label to indicate that this machine is broken
		if machine.ObjectMeta.Annotations == nil {
			machine.ObjectMeta.Annotations = make(map[string]string)
		}
		machine.ObjectMeta.Annotations[MachineInstanceStateAnnotationName] = ErrorState

		if err := oc.client.Update(context.TODO(), machine); err != nil {
			return fmt.Errorf("unable to update machine status: %v", err)
		}
	}

	klog.Errorf("Machine error %s: %v", machine.Name, err.Message)
	return err
}

func (oc *OpenstackClient) updateAnnotation(machine *machinev1.Machine, instance *clients.Instance, clusterInfraName string) error {
	providerID := fmt.Sprintf("openstack:///%s", instance.ID)

	if machine.Spec.ProviderID != nil {
		// We can't recover if the provider ID has changed
		if *machine.Spec.ProviderID != providerID {
			verr := apierrors.InvalidMachineConfiguration("providerID has changed from %s to %s. This is not supported. "+
				"The recommended action is to delete and recreate this machine.", *machine.Spec.ProviderID, providerID)
			return oc.handleMachineError(machine, verr, updateEventAction)
		}
	} else {
		machine.Spec.ProviderID = &providerID
	}

	statusCopy := *machine.Status.DeepCopy()

	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}
	machine.ObjectMeta.Annotations[OpenstackIdAnnotationKey] = instance.ID
	machine.ObjectMeta.Annotations[MachineInstanceStateAnnotationName] = instance.Status

	if err := oc.client.Update(context.TODO(), machine); err != nil {
		return err
	}

	nodeAddresses, err := getIPsFromInstance(instance)
	if err != nil {
		return err
	}

	nodeAddresses = append(nodeAddresses, corev1.NodeAddress{
		Type:    corev1.NodeHostName,
		Address: machine.Name,
	})

	nodeAddresses = append(nodeAddresses, corev1.NodeAddress{
		Type:    corev1.NodeInternalDNS,
		Address: machine.Name,
	})

	machineCopy := machine.DeepCopy()
	machineCopy.Status.Addresses = nodeAddresses

	if !equality.Semantic.DeepEqual(machine.Status.Addresses, machineCopy.Status.Addresses) {
		if err := oc.client.Status().Update(context.TODO(), machineCopy); err != nil {
			return err
		}
	}

	machine.Status = statusCopy
	return oc.client.Update(context.TODO(), machine)
}

func (oc *OpenstackClient) instanceExists(machine *machinev1.Machine) (instance *clients.Instance, err error) {
	machineSpec, err := openstackconfigv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return nil, fmt.Errorf("\nError getting the machine spec from the provider spec (machine/actuator.go 457): %v", err)
	}
	opts := &clients.InstanceListOpts{
		Name:   machine.Name,
		Image:  machineSpec.Image,
		Flavor: machineSpec.Flavor,
	}

	machineService, err := clients.NewInstanceServiceFromMachine(oc.params.KubeClient, machine)
	if err != nil {
		return nil, fmt.Errorf("\nError getting a new instance service from the machine (machine/actuator.go 467): %v", err)
	}

	instanceList, err := machineService.GetInstanceList(opts)
	if err != nil {
		return nil, fmt.Errorf("\nError listing the instances: %v", err)
	}
	if len(instanceList) == 0 {
		return nil, nil
	}
	return instanceList[0], nil
}

func (oc *OpenstackClient) createBootstrapToken() (string, error) {
	token, err := tokenutil.GenerateBootstrapToken()
	if err != nil {
		return "", err
	}

	expiration := time.Now().UTC().Add(options.TokenTTL)
	tokenSecret, err := bootstrap.GenerateTokenSecret(token, expiration)
	if err != nil {
		panic(fmt.Sprintf("unable to create token. there might be a bug somwhere: %v", err))
	}

	err = oc.client.Create(context.TODO(), tokenSecret)
	if err != nil {
		return "", err
	}

	return tokenutil.TokenFromIDAndSecret(
		string(tokenSecret.Data[tokenapi.BootstrapTokenIDKey]),
		string(tokenSecret.Data[tokenapi.BootstrapTokenSecretKey]),
	), nil
}

func (oc *OpenstackClient) validateMachine(machine *machinev1.Machine) error {
	machineSpec, err := openstackconfigv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return fmt.Errorf("\nError getting the machine spec from the provider spec: %v", err)
	}

	machineService, err := clients.NewInstanceServiceFromMachine(oc.params.KubeClient, machine)
	if err != nil {
		return fmt.Errorf("\nError getting a new instance service from the machine: %v", err)
	}

	// TODO(mfedosin): add more validations here

	// Validate that image exists when not booting from volume
	if machineSpec.RootVolume == nil {
		err = machineService.DoesImageExist(machineSpec.Image)
		if err != nil {
			return err
		}
	}

	// Validate that flavor exists
	err = machineService.DoesFlavorExist(machineSpec.Flavor)
	if err != nil {
		return err
	}

	// Validate that Availability Zone exists
	err = machineService.DoesAvailabilityZoneExist(machineSpec.AvailabilityZone)
	if err != nil {
		return err
	}

	return nil
}
