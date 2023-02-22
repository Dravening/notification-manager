/*


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
	"github.com/d3os/notification-manager/pkg/apis/v2beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	notificationManager       = "notification-manager"
	defaultPortName           = "webhook"
	defaultServiceAccountName = "default"
)

var (
	ownerKey               = ".metadata.controller"
	apiGVStr               = v2beta1.GroupVersion.String()
	log                    logr.Logger
	minReplicas            int32 = 1
	defaultImage                 = "d3os/notification-manager:v1.0.0"
	defaultImagePullPolicy       = corev1.PullIfNotPresent
)

// NotificationManagerReconciler reconciles a NotificationManager object
type NotificationManagerReconciler struct {
	client.Client
	Namespace string
	Log       logr.Logger
	Scheme    *runtime.Scheme
}

// Reconcile reads that state of NotificationManager objects and makes changes based on the state read
// and what is in the NotificationManagerSpec
// +kubebuilder:rbac:groups=notification.d3os.io,resources=notificationmanagers;receivers;configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=notification.d3os.io,resources=notificationmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;

func (r *NotificationManagerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log = r.Log.WithValues("NotificationManager Operator", req.NamespacedName)

	var nm v2beta1.NotificationManager
	if err := r.Get(ctx, req.NamespacedName, &nm); err != nil {
		log.Error(err, "Unable to get NotificationManager", "Req", req.NamespacedName.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var err error
	result := controllerutil.OperationResultNone

	// Create deployment service
	if err = r.createDeploymentSvc(ctx, &nm); err != nil {
		log.Error(err, "Failed to create svc")
		return ctrl.Result{}, err
	}

	// Create or update deployment
	result = controllerutil.OperationResultNone
	deploy := &appsv1.Deployment{}
	deploy.ObjectMeta.Name = nm.Name + "-deployment"
	deploy.ObjectMeta.Namespace = r.Namespace
	if result, err = controllerutil.CreateOrUpdate(ctx, r.Client, deploy, r.mutateDeployment(deploy, &nm)); err != nil {
		log.Error(err, "Failed to CreateOrUpdate deployment", "result", result)
		return ctrl.Result{}, err
	}
	log.V(10).Info("CreateOrUpdate deployment returns", "result", result)

	return ctrl.Result{}, nil
}

func (r *NotificationManagerReconciler) createDeploymentSvc(ctx context.Context, nm *v2beta1.NotificationManager) error {
	nm = nm.DeepCopy()
	if nm.Spec.PortName == "" {
		nm.Spec.PortName = defaultPortName
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nm.Name + "-svc",
			Namespace: r.Namespace,
			Labels:    *r.makeCommonLabels(nm),
		},
		Spec: corev1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []corev1.ServicePort{
				{
					Name:       nm.Spec.PortName,
					Port:       19093,
					TargetPort: intstr.FromString(nm.Spec.PortName),
				},
			},
			Selector: *r.makeCommonLabels(nm),
		},
	}

	if err := ctrl.SetControllerReference(nm, svc, r.Scheme); err != nil {
		log.Error(err, "SetControllerReference failed for service")
		return err
	}
	if err := r.Create(ctx, svc); err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "Create service failed")
		return err
	}

	return nil
}

func (r *NotificationManagerReconciler) mutateDeployment(deploy *appsv1.Deployment, nm *v2beta1.NotificationManager) controllerutil.MutateFn {
	return func() error {
		nm = nm.DeepCopy()

		if nm.Spec.Image == nil || nm.Spec.Image != nil && *nm.Spec.Image == "" {
			nm.Spec.Image = &defaultImage
		}

		if nm.Spec.ImagePullPolicy == nil || nm.Spec.ImagePullPolicy != nil && *nm.Spec.ImagePullPolicy == "" {
			nm.Spec.ImagePullPolicy = &defaultImagePullPolicy
		}

		if nm.Spec.Replicas == nil || nm.Spec.Replicas != nil && *nm.Spec.Replicas <= int32(0) {
			nm.Spec.Replicas = &minReplicas
		}

		if nm.Spec.PortName == "" {
			nm.Spec.PortName = defaultPortName
		}

		if nm.Spec.ServiceAccountName == "" {
			nm.Spec.ServiceAccountName = defaultServiceAccountName
		}

		deploy.ObjectMeta.Labels = *r.makeCommonLabels(nm)
		deploy.Spec.Replicas = nm.Spec.Replicas
		podLabels := deploy.ObjectMeta.Labels
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: podLabels,
		}
		deploy.Spec.Template.ObjectMeta = metav1.ObjectMeta{
			Labels: podLabels,
		}
		deploy.Spec.Template.Spec.NodeSelector = nm.Spec.NodeSelector
		deploy.Spec.Template.Spec.Affinity = nm.Spec.Affinity
		deploy.Spec.Template.Spec.Tolerations = nm.Spec.Tolerations
		deploy.Spec.Template.Spec.ServiceAccountName = nm.Spec.ServiceAccountName

		// Define expected container
		newC := corev1.Container{
			Name:            notificationManager,
			Resources:       nm.Spec.Resources,
			Image:           *nm.Spec.Image,
			ImagePullPolicy: *nm.Spec.ImagePullPolicy,
			Ports: []corev1.ContainerPort{
				{
					Name:          nm.Spec.PortName,
					ContainerPort: 19093,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "host-time",
					MountPath: "/etc/localtime",
				},
			},
		}

		if nm.Spec.DefaultSecretNamespace == "" {
			newC.Env = []corev1.EnvVar{
				{
					Name: "NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				},
			}
		} else {
			newC.Env = []corev1.EnvVar{
				{
					Name:  "NAMESPACE",
					Value: nm.Spec.DefaultSecretNamespace,
				},
			}
		}

		if nm.Spec.VolumeMounts != nil {
			newC.VolumeMounts = append(newC.VolumeMounts, nm.Spec.VolumeMounts...)
		}

		if nm.Spec.Args != nil {
			newC.Args = append(newC.Args, nm.Spec.Args...)
		}

		// Make sure existing Containers match expected Containers
		for i, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == newC.Name {
				deploy.Spec.Template.Spec.Containers[i].Resources = newC.Resources
				deploy.Spec.Template.Spec.Containers[i].Image = newC.Image
				deploy.Spec.Template.Spec.Containers[i].ImagePullPolicy = newC.ImagePullPolicy
				deploy.Spec.Template.Spec.Containers[i].Ports = newC.Ports
				deploy.Spec.Template.Spec.Containers[i].Command = newC.Command
				deploy.Spec.Template.Spec.Containers[i].Env = newC.Env
				deploy.Spec.Template.Spec.Containers[i].VolumeMounts = newC.VolumeMounts
				deploy.Spec.Template.Spec.Containers[i].Args = newC.Args
				break
			}
		}

		// Create new Containers if no existing Containers exist
		if len(deploy.Spec.Template.Spec.Containers) == 0 {
			deploy.Spec.Template.Spec.Containers = []corev1.Container{newC}
		}

		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "host-time",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/localtime",
					},
				},
			},
		}
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, nm.Spec.Volumes...)

		deploy.SetOwnerReferences(nil)
		return ctrl.SetControllerReference(nm, deploy, r.Scheme)
	}
}

func (r *NotificationManagerReconciler) makeCommonLabels(nm *v2beta1.NotificationManager) *map[string]string {
	return &map[string]string{"app": notificationManager, notificationManager: nm.Name}
}

func (r *NotificationManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(&corev1.Service{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the service object, extract the owner.
		svc := rawObj.(*corev1.Service)
		owner := metav1.GetControllerOf(svc)
		if owner == nil {
			return nil
		}
		// Make sure it's a NotificationManager. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "NotificationManager" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(&appsv1.Deployment{}, ownerKey, func(rawObj runtime.Object) []string {
		// grab the deployment object, extract the owner.
		deploy := rawObj.(*appsv1.Deployment)
		owner := metav1.GetControllerOf(deploy)
		if owner == nil {
			return nil
		}
		// Make sure it's a NotificationManager. If so, return it.
		if owner.APIVersion != apiGVStr || owner.Kind != "NotificationManager" {
			return nil
		}
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v2beta1.NotificationManager{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
