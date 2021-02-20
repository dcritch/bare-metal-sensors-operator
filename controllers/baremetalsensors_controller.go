/*
Copyright 2021.

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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sensorsv1alpha1 "github.com/dcritch/bare-metal-sensors-operator/api/v1alpha1"
)

// BareMetalSensorsReconciler reconciles a BareMetalSensors object
type BareMetalSensorsReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=sensors.xana.du,resources=baremetalsensors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sensors.xana.du,resources=baremetalsensors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sensors.xana.du,resources=baremetalsensors/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BareMetalSensors object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *BareMetalSensorsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("baremetalsensors", req.NamespacedName)

	// Fetch the BareMetalSensors instance
	baremetalsensors := &sensorsv1alpha1.BareMetalSensors{}
	err := r.Get(ctx, req.NamespacedName, baremetalsensors)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("BareMetalSensors resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get BareMetalSensors")
		return ctrl.Result{}, err
	}

	sa := r.saForBareMetalSensors(baremetalsensors)
	log.Info("Creating a new ServiceAccount", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
	err = r.Create(ctx, sa)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Failed to create new ServiceAccount", "ServiceAccount.Namespace", sa.Namespace, "ServiceAccount.Name", sa.Name)
		return ctrl.Result{}, err
	}

	role := r.saRole(baremetalsensors)
	log.Info("Creating new ClusterRoleBinding", "ServiceAccount.Namespace", role.Namespace, "ServiceAccount.Name", role.Name)
	err = r.Create(ctx, role)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Failed to create new ClusterRoleBinding", "ClusterRoleBinding.Namespace", role.Namespace, "ClusterRoleBinding.Name", role.Name)
		return ctrl.Result{}, err
	}

	cm := r.configmapForBareMetalSensors(baremetalsensors)
	log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
	err = r.Create(ctx, cm)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		return ctrl.Result{}, err
	}

	// Check if the daemonset already exists, if not create a new one
	found := &appsv1.DaemonSet{}
	err = r.Get(ctx, types.NamespacedName{Name: baremetalsensors.Name, Namespace: baremetalsensors.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new daemonset
		ds := r.daemonsetForBareMetalSensors(baremetalsensors)
		log.Info("Creating a new DaemonSet", "DaemonSet.Namespace", ds.Namespace, "DaemonSet.Name", ds.Name)
		err = r.Create(ctx, ds)
		if err != nil {
			log.Error(err, "Failed to create new DaemonSet", "DaemonSet.Namespace", ds.Namespace, "DaemonSet.Name", ds.Name)
			return ctrl.Result{}, err
		}
		// DaemonSet created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get DaemonSet")
		return ctrl.Result{}, err
	}

	// Update the BareMetalSensors status with the pod names
	// List the pods for this baremetalsensors's deployment
	podList := &corev1.PodList{}
	podListOpts := []client.ListOption{
		client.InNamespace(baremetalsensors.Namespace),
		client.MatchingLabels(labelsForBareMetalSensors(baremetalsensors.Name)),
	}
	if err = r.List(ctx, podList, podListOpts...); err != nil {
		log.Error(err, "Failed to list pods", "BareMetalSensors.Namespace", baremetalsensors.Namespace, "BareMetalSensors.Name", baremetalsensors.Name)
		return ctrl.Result{}, err
	}
	podNames := getPodNames(podList.Items)

	// Update status.Nodes if needed
	if !reflect.DeepEqual(podNames, baremetalsensors.Status.Nodes) {
		baremetalsensors.Status.Nodes = podNames
		err := r.Status().Update(ctx, baremetalsensors)
		if err != nil {
			log.Error(err, "Failed to update BareMetalSensors status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// configmapForBareMetalSensors returns a baremetalsensors config-map object
func (r BareMetalSensorsReconciler) configmapForBareMetalSensors(b *sensorsv1alpha1.BareMetalSensors) *corev1.ConfigMap {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "collectd-config-include",
			Namespace: b.Namespace,
		},
		Data: configForBareMetalSensors(b.Spec.InfluxHost),
	}
	ctrl.SetControllerReference(b, cm, r.Scheme)
	return cm
}

// saForBareMetalSensors returns a baremetalsensors serviceaccount object
func (r BareMetalSensorsReconciler) saForBareMetalSensors(b *sensorsv1alpha1.BareMetalSensors) *corev1.ServiceAccount {

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bare-metal-sensors",
			Namespace: b.Namespace,
		},
	}
	ctrl.SetControllerReference(b, sa, r.Scheme)
	return sa
}

// saRole returns a ClusterRoleBinding object
func (r BareMetalSensorsReconciler) saRole(b *sensorsv1alpha1.BareMetalSensors) *rbac.ClusterRoleBinding {

	role := &rbac.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Namespace + "hack",
		},
		Subjects: []rbac.Subject{
			{
				Name:      "bare-metal-sensors",
				Kind:      "ServiceAccount",
				Namespace: b.Namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}
	ctrl.SetControllerReference(b, role, r.Scheme)
	return role

}

// daemonsetForBareMetalSensors returns a baremetalsensors DaemonSet object
func (r BareMetalSensorsReconciler) daemonsetForBareMetalSensors(b *sensorsv1alpha1.BareMetalSensors) *appsv1.DaemonSet {
	ls := labelsForBareMetalSensors(b.Name)
	//rootID := int64(0)
	privileged := bool(true)

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.Name,
			Namespace: b.Namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:       ls,
					GenerateName: "bms",
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "bare-metal-sensors",
					Volumes: []corev1.Volume{
						{
							Name: "collectd-config-include",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "collectd-config-include",
									},
								},
							},
						},
						{
							Name: "rootfs",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
								},
							},
						},
					},
					Containers: []corev1.Container{{
						Image:   "quay.io/dcritch/bms:latest",
						Name:    "bare-metal-sensors",
						Command: []string{"/opt/collectd-wrapper.sh"},
						SecurityContext: &corev1.SecurityContext{
							Privileged: &privileged,
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "collectd-config-include",
								MountPath: "/etc/collectd.d",
							},
							{
								Name:      "rootfs",
								MountPath: "/rootfs",
							},
						},
						Env: []corev1.EnvVar{{
							Name: "HOSTNAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "spec.nodeName",
								},
							},
						}},
					}},
				},
			},
		},
	}
	// Set BareMetalSensors instance as the owner and controller
	ctrl.SetControllerReference(b, ds, r.Scheme)
	return ds
}

func configForBareMetalSensors(influxdbHost string) map[string]string {
	cmData := make(map[string]string, 0)

	commonConf := "FQDNLookup false\nInterval 60\n"
	sensorsConf := "LoadPlugin sensors\n"
	influxConf := "LoadPlugin network\n<Plugin network>\nServer \"" + influxdbHost + "\" \"25826\"\n</Plugin>\n"
	prometheusConf := "LoadPlugin write_prometheus\n<Plugin write_prometheus>\nPort \"9103\"\n</Plugin>\n"
	hostnameConf := "Include \"/tmp/collectd-hostname\"\n"
	nvmeConf := "LoadPlugin exec\n<Plugin exec>\nExec \"nvme-sensors:nvme-sensors\" \"/opt/smart-nvme.sh\"\n</Plugin>\n"

	cmData["common.conf"] = commonConf
	cmData["sensors.conf"] = sensorsConf
	cmData["influx.conf"] = influxConf
	cmData["prometheus.conf"] = prometheusConf
	cmData["hostname.conf"] = hostnameConf
	cmData["smart-nvme.conf"] = nvmeConf
	return cmData
}

// labelsForBareMetalSensors returns the labels for selecting the resources
// belonging to the given baremetalsensors CR name.
func labelsForBareMetalSensors(name string) map[string]string {
	return map[string]string{"app": "bare-metal-sensors", "baremetalsensors_cr": name}
}

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// SetupWithManager sets up the controller with the Manager.
func (r *BareMetalSensorsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sensorsv1alpha1.BareMetalSensors{}).
		Complete(r)
}
