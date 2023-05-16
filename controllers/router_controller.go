/*
Copyright 2023.

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

package controllers

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"net"
	"reflect"
	"strings"

	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kvnetv1alpha1 "github.com/tjjh89017/kvnet/api/v1alpha1"
)

const (
	InitContainerCommand = `
		ip link add mgmt type vrf table 10
		ip link set mgmt up
		ip route add table 10 unreachable default metric 4278198272
		ip route > route.txt
		ip link set eth0 master mgmt
		cat route.txt | xargs -t -I {} sh -c "ip route add table 10 {}"
	`
	WatchConfigMapCommand = `
		sh "$(readlink -f "/config/ip-setup.sh")"
		while true
		do
			REAL=$(readlink -f "/config/ip-setup.sh")
			echo "wait for change"
			inotifywait -e delete_self "${REAL}"
			echo "configMap change"
			sh "${REAL}"
		done
	`
)

// RouterReconciler reconciles a Router object
type RouterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=routers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=routers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=routers/finalizers,verbs=update
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kvnet.kojuro.date,resources=subnets/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Router object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *RouterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling Router")

	router := &kvnetv1alpha1.Router{}
	if err := r.Get(ctx, req.NamespacedName, router); err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Router resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		reqLogger.Error(err, "Failed to get Router")
		return ctrl.Result{}, err
	}

	if router.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(router, kvnetv1alpha1.RouterFinalizer) {
			logrus.Info("already onRemove")
			return ctrl.Result{}, nil
		}

		if result, err := r.OnRemove(ctx, router); err != nil {
			return result, err
		}

		controllerutil.RemoveFinalizer(router, kvnetv1alpha1.RouterFinalizer)
		if err := r.Update(ctx, router); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	return r.OnChange(ctx, router)
}

func (r *RouterReconciler) OnChange(ctx context.Context, router *kvnetv1alpha1.Router) (ctrl.Result, error) {
	logrus.Infof("Router OnChange")

	// check subnets to update labels
	if err := r.updateSubnetLabel(ctx, router); err != nil {
		return ctrl.Result{}, err
	}

	// override all subnets
	subnets, err := r.getSubnetListWithRouter(ctx, router)
	if err != nil {
		logrus.Errorf("get updated subnet failed %v", err)
		return ctrl.Result{}, err
	}

	// cretae or update configMap
	// TODO custom route for FRR
	if err := r.updateConfigMap(ctx, router, subnets); err != nil {
		logrus.Errorf("create or update configmap fail %v", err)
		return ctrl.Result{}, err
	}

	// create or update deployment
	if err := r.updateDeployment(ctx, router, subnets); err != nil {
		logrus.Errorf("create or update deployment fail %v", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RouterReconciler) OnRemove(ctx context.Context, router *kvnetv1alpha1.Router) (ctrl.Result, error) {
	logrus.Infof("Router OnRemove")

	// remove all deployment
	deployment := &appv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: router.Name}, deployment); err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("remove: get deployment failed %v", err)
			return ctrl.Result{}, err
		}
	} else if err := r.Delete(ctx, deployment); err != nil {
		logrus.Errorf("remove: delete deployment failed %v", err)
		return ctrl.Result{}, err
	}

	// remove all configMap
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: router.Name}, configMap); err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("remove: get configMap failed %v", err)
			return ctrl.Result{}, err
		}
	} else if err := r.Delete(ctx, configMap); err != nil {
		logrus.Errorf("remove: delete configMap failed %v", err)
		return ctrl.Result{}, err
	}

	// remove subnets labels
	for _, subnetName := range router.Spec.Subnets {
		subnet := &kvnetv1alpha1.Subnet{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: subnetName.Name}, subnet); err != nil {
			if !errors.IsNotFound(err) {
				logrus.Errorf("remove: get subnet failed %s", err)
				return ctrl.Result{}, err
			}
		} else if subnet.Labels[kvnetv1alpha1.RouterSubnetOwnerLabel] == router.Name {
			// found
			subnetCopy := subnet.DeepCopy()
			delete(subnetCopy.Labels, kvnetv1alpha1.RouterSubnetOwnerLabel)
			if !reflect.DeepEqual(subnet, subnetCopy) {
				if err := r.Update(ctx, subnetCopy); err != nil {
					logrus.Errorf("remove: delete subnet labels failed %v", err)
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *RouterReconciler) updateDeployment(ctx context.Context, router *kvnetv1alpha1.Router, subnets []*kvnetv1alpha1.Subnet) error {
	needCreate := false
	deployment := &appv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: router.Name}, deployment); err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("get deployment failed %v", err)
			return err
		}
		needCreate = true
		replicas := int32(1)
		privileged := true
		routerKind := reflect.TypeOf(kvnetv1alpha1.Router{}).Name()
		deployment = &appv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: router.Namespace,
				Name:      router.Name,
				Labels: map[string]string{
					"app": router.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(router, kvnetv1alpha1.GroupVersion.WithKind(routerKind)),
				},
			},
			Spec: appv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": router.Name,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": router.Name,
						},
					},
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{
								Name:            "init-container",
								Image:           "tjjh89017/alpine-nettools",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/sh",
									"-c",
									InitContainerCommand,
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: &privileged,
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "config-map-notify",
								Image:           "tjjh89017/alpine-nettools",
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command: []string{
									"/bin/sh",
									"-c",
									WatchConfigMapCommand,
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: &privileged,
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "config",
										MountPath: "/config",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: router.Name,
										},
									},
								},
							},
						},
					},
				},
			},
		}
	}

	deploymentCopy := deployment.DeepCopy()

	// replace multus annotation
	nadList := make([]string, 0)
	for _, subnet := range subnets {
		nadList = append(nadList, subnet.Spec.Network)
	}
	nadName := strings.Join(nadList[:], ", ")
	annotations := deploymentCopy.Spec.Template.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["k8s.v1.cni.cncf.io/networks"] = nadName
	deploymentCopy.Spec.Template.Annotations = annotations
	logrus.Infof("deployment template %v", deploymentCopy.Spec.Template)

	// TODO replace affinity
	matchExpressions := make([]corev1.NodeSelectorRequirement, 0)
	for _, subnet := range subnets {
		for k, _ := range subnet.Labels {
			if strings.HasPrefix(k, kvnetv1alpha1.BridgeNodeLabel) {
				matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
					Key:      k,
					Operator: corev1.NodeSelectorOpExists,
				})
			}
		}
	}

	deploymentCopy.Spec.Template.Spec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: matchExpressions,
					},
				},
			},
		},
	}

	if needCreate {
		if err := r.Create(ctx, deploymentCopy); err != nil {
			logrus.Errorf("error to create router deployment %v", err)
			return err
		}
	} else {
		if !reflect.DeepEqual(deployment, deploymentCopy) {
			if err := r.Update(ctx, deploymentCopy); err != nil {
				logrus.Errorf("error to update router deployment %v", err)
				return err
			}
		}
	}

	return nil
}

func (r *RouterReconciler) updateConfigMap(ctx context.Context, router *kvnetv1alpha1.Router, subnets []*kvnetv1alpha1.Subnet) error {
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: router.Name}, configMap); err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("get configMap failed %v", err)
			return err
		}
		routerKind := reflect.TypeOf(kvnetv1alpha1.Router{}).Name()
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: router.Namespace,
				Name:      router.Name,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(router, kvnetv1alpha1.GroupVersion.WithKind(routerKind)),
				},
			},
		}

		if err := r.Create(ctx, configMap); err != nil {
			logrus.Errorf("create default configMap fail %v", err)
			return err
		}

		// TODO not sure we need this again
		if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: router.Name}, configMap); err != nil {
			logrus.Errorf("get configMap failed again %v", err)
			return err
		}
	}

	configMapCopy := configMap.DeepCopy()

	if configMapCopy.Data == nil {
		configMapCopy.Data = make(map[string]string)
	}

	logrus.Infof("configmap subnets %v", subnets)
	// generate interface IP config
	ipCmd := ""
	if len(router.Spec.Subnets) != len(subnets) {
		logrus.Errorf("subnet is not ready. reconcile")
		return fmt.Errorf("subnet is not ready. reconcile")
	}
	for i, subnet := range router.Spec.Subnets {
		switch subnet.IPMode {
		case "":
			// get router ip from kvnetv1alpha1.Subnet
			routerIP := net.ParseIP(subnets[i].Spec.RouterIP)
			if routerIP == nil {
				logrus.Errorf("parse routerIP from subnet CR fail")
				return fmt.Errorf("parse routerIP from subnet CR fail")
			}
			_, ipv4Net, err := net.ParseCIDR(subnets[i].Spec.NetworkCIDR)
			if err != nil {
				logrus.Errorf("parse networkCIDR from subnet CR fail %v", err)
				return err
			}

			ipv4Net.IP = routerIP

			ipCmd = ipCmd + fmt.Sprintf("ip addr flush dev net%d\n", i+1)
			ipCmd = ipCmd + fmt.Sprintf("ip addr add %s dev net%d\n", ipv4Net.String(), i+1)

		case "static":
			// TODO implement
		case "dhcp":
			// TODO implement
		}

	}

	logrus.Infof("ipCmd: %s", ipCmd)

	configMapCopy.Data["ip-setup.sh"] = ipCmd

	// TODO FRR static route

	if !reflect.DeepEqual(configMap, configMapCopy) {
		if err := r.Update(ctx, configMapCopy); err != nil {
			logrus.Errorf("update configMap failed %v", err)
			return err
		}
	}

	return nil
}

func (r *RouterReconciler) getSubnetListWithRouter(ctx context.Context, router *kvnetv1alpha1.Router) ([]*kvnetv1alpha1.Subnet, error) {
	// get all labels with current router name
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			kvnetv1alpha1.RouterSubnetOwnerLabel: router.Name,
		},
	})
	if err != nil {
		logrus.Errorf("error to create router selector, %v", err)
		return nil, err
	}

	subnetList := &kvnetv1alpha1.SubnetList{}
	if err := r.List(ctx, subnetList, client.InNamespace(router.Namespace), client.MatchingLabelsSelector{Selector: selector}); err != nil {
		logrus.Errorf("get subnets failed %v", err)
		return nil, err
	}

	subnets := make([]*kvnetv1alpha1.Subnet, 0)
	for _, subnetOption := range router.Spec.Subnets {
		// search
		for _, subnet := range subnetList.Items {
			if subnet.Name == subnetOption.Name {
				subnets = append(subnets, &subnet)
				break
			}
		}
	}

	return subnets, nil
}

func (r *RouterReconciler) updateSubnetLabel(ctx context.Context, router *kvnetv1alpha1.Router) error {
	subnets, err := r.getSubnetListWithRouter(ctx, router)
	if err != nil {
		return err
	}

	subnetMap := make(map[string]int)
	for _, subnet := range router.Spec.Subnets {
		// set 1 to mark we should create but we still not
		subnetMap[subnet.Name] = 1
	}
	for _, subnet := range subnets {
		// already have this subnet
		subnetMap[subnet.Name] |= 2
	}

	for subnetName, state := range subnetMap {
		switch state {
		case 1:
			// need create
			subnet := &kvnetv1alpha1.Subnet{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: subnetName}, subnet); err != nil {
				logrus.Errorf("get subnet to add router label failed %v", err)
				return err
			}

			if subnet.Labels == nil {
				subnet.Labels = make(map[string]string)
			}

			// TODO check subnet.Labels[kvnetv1alpha1.RouterSubnetOwnerLabel] value befor setting

			subnet.Labels[kvnetv1alpha1.RouterSubnetOwnerLabel] = router.Name
			if err := r.Update(ctx, subnet); err != nil {
				logrus.Errorf("add router label to subnet fail %v", err)
				return err
			}
		case 2:
			// need delete
			subnet := &kvnetv1alpha1.Subnet{}
			if err := r.Get(ctx, types.NamespacedName{Namespace: router.Namespace, Name: subnetName}, subnet); err != nil {
				logrus.Errorf("get subnet to delete router label failed %v", err)
				return err
			}

			delete(subnet.Labels, kvnetv1alpha1.RouterSubnetOwnerLabel)
			if err := r.Update(ctx, subnet); err != nil {
				logrus.Errorf("del router label to subnet fail %v", err)
				return err
			}
		case 3:
			// override config
			// do nothing
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RouterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kvnetv1alpha1.Router{}).
		Complete(r)
	// TODO watch subnet change
	// TODO watch deployment to redeploy
}
