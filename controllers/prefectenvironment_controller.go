/*
Copyright 2023 Willem Meints.

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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	utils "github.com/wmeints/prefect-operator/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	mlopsv1alpha1 "github.com/wmeints/prefect-operator/api/v1alpha1"
)

// PrefectEnvironmentReconciler reconciles a PrefectEnvironment object
type PrefectEnvironmentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mlops.aigency.com,resources=prefectenvironments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mlops.aigency.com,resources=prefectenvironments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mlops.aigency.com,resources=prefectenvironments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PrefectEnvironment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *PrefectEnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	environment := &mlopsv1alpha1.PrefectEnvironment{}

	if err := r.Get(ctx, req.NamespacedName, environment); err != nil {
		logger.Info("unable to fetch PrefectEnvironment, ignoring the operation.")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("reconciling PrefectEnvironment", "name", environment.Name)

	if err := r.reconcileEnvironmentSecrets(ctx, environment); err != nil {
		logger.Error(err, "unable to reconcile environment secrets")
		return ctrl.Result{}, err
	}

	if err := r.reconcileOrionDatabase(ctx, environment); err != nil {
		logger.Error(err, "unable to reconcile the orion database")
		return ctrl.Result{}, err
	}

	if err := r.reconcileOrionServer(ctx, environment); err != nil {
		logger.Error(err, "unable to reconcile the orion server")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectEnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mlopsv1alpha1.PrefectEnvironment{}).
		Complete(r)
}

func (r *PrefectEnvironmentReconciler) reconcileEnvironmentSecrets(ctx context.Context, environment *mlopsv1alpha1.PrefectEnvironment) error {
	logger := log.FromContext(ctx)
	secretName := fmt.Sprintf("%s-environment-secrets", environment.Name)
	secret := &corev1.Secret{}

	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: environment.GetNamespace()}, secret); err != nil {
		if errors.IsNotFound(err) {
			databasePassword := utils.GeneratePassword(16, 2, 2, 2)

			logger.Info("automatically generating secret for the environment", "name", environment.GetName())

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: environment.GetNamespace(),
				},
				StringData: map[string]string{
					"databasePassword": databasePassword,
				},
			}

			ctrl.SetControllerReference(environment, secret, r.Scheme)

			if err := r.Create(ctx, secret); err != nil {
				return err
			}

			return nil
		}

		return err
	}

	return nil
}

func (r *PrefectEnvironmentReconciler) reconcileOrionDatabase(ctx context.Context, environment *mlopsv1alpha1.PrefectEnvironment) error {
	if err := r.reconcileOrionDatabaseDeployment(ctx, environment); err != nil {
		return err
	}

	if err := r.reconcileOrionDatabaseService(ctx, environment); err != nil {
		return err
	}

	return nil
}

func (r *PrefectEnvironmentReconciler) reconcileOrionDatabaseDeployment(ctx context.Context, environment *mlopsv1alpha1.PrefectEnvironment) error {
	logger := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("%s-orion-database", environment.Name)
	deployment := &appsv1.Deployment{}

	deploymentLabels := map[string]string{
		"mlops.aigency.com/environment": environment.Name,
		"mlops.aigency.com/component":   "orion-database",
	}

	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: environment.GetNamespace()}, deployment); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating orion database", "replicas", environment.Spec.DatabaseReplicas, "name", deploymentName)

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: environment.GetNamespace(),
					Labels:    deploymentLabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &environment.Spec.DatabaseReplicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: deploymentLabels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: deploymentLabels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "orion-database",
									Image: "postgres:14",
									Env: []corev1.EnvVar{
										{
											Name: "POSTGRES_PASSWORD",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: fmt.Sprintf("%s-environment-secrets", environment.Name),
													},
													Key: "databasePassword",
												},
											},
										},
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 5432,
											Name:          "tcp-postgres",
										},
									},
								},
							},
						},
					},
				},
			}

			if err := ctrl.SetControllerReference(environment, deployment, r.Scheme); err != nil {
				return err
			}

			if err := r.Create(ctx, deployment); err != nil {
				return err
			}

			return nil
		}

		if deployment.Spec.Replicas != &environment.Spec.DatabaseReplicas {
			deployment.Spec.Replicas = &environment.Spec.DatabaseReplicas

			logger.Info("scaling orion database", "replicas", environment.Spec.DatabaseReplicas, "name", deploymentName)

			if err := r.Update(ctx, deployment); err != nil {
				return nil
			}
		}

		return nil
	}

	return nil
}

func (r *PrefectEnvironmentReconciler) reconcileOrionDatabaseService(ctx context.Context, environment *mlopsv1alpha1.PrefectEnvironment) error {
	logger := log.FromContext(ctx)

	serviceLabels := map[string]string{
		"mlops.aigency.com/environment": environment.Name,
		"mlops.aigency.com/component":   "orion-database",
	}

	serviceName := fmt.Sprintf("%s-orion-database", environment.Name)
	service := &corev1.Service{}

	if err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: environment.GetNamespace()}, service); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating orion database service", "name", serviceName)

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: environment.GetNamespace(),
					Labels:    serviceLabels,
				},
				Spec: corev1.ServiceSpec{
					Selector: serviceLabels,
					Type:     corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							TargetPort: intstr.FromInt(5432),
							Port:       5432,
							Name:       "tcp-postgres",
						},
					},
				},
			}

			if err := ctrl.SetControllerReference(environment, service, r.Scheme); err != nil {
				return err
			}

			if err := r.Create(ctx, service); err != nil {
				return err
			}

			return nil
		}

		return err
	}

	return nil
}

func (r *PrefectEnvironmentReconciler) reconcileOrionServer(ctx context.Context, environment *mlopsv1alpha1.PrefectEnvironment) error {
	if err := r.reconcileOrionServerDeployment(ctx, environment); err != nil {
		return err
	}

	if err := r.reconcileOrionServerService(ctx, environment); err != nil {
		return err
	}

	return nil
}

func (r *PrefectEnvironmentReconciler) reconcileOrionServerDeployment(ctx context.Context, environment *mlopsv1alpha1.PrefectEnvironment) error {
	logger := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("%s-orion-server", environment.Name)
	deployment := &appsv1.Deployment{}

	deploymentImage := environment.Spec.Image

	// Automatically choose the latest when we don't get an image from the user.
	if deploymentImage == "" {
		deploymentImage = "prefecthq/prefect:2-latest"
	}

	if err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: environment.GetNamespace()}, deployment); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating orion server deployment", "name", deploymentName)

			deploymentLabels := map[string]string{
				"mlops.aigency.com/environment": environment.Name,
				"mlops.aigency.com/component":   "orion-server",
			}

			deployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: environment.GetNamespace(),
					Labels:    deploymentLabels,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &environment.Spec.OrionReplicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: deploymentLabels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: deploymentLabels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "orion-server",
									Image: deploymentImage,
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 4200,
											Name:          "http-orion",
										},
									},
								},
							},
						},
					},
				},
			}

			if err := ctrl.SetControllerReference(environment, deployment, r.Scheme); err != nil {
				return err
			}

			if err := r.Create(ctx, deployment); err != nil {
				return err
			}

			return nil
		}

		return err
	}

	if deployment.Spec.Replicas != &environment.Spec.OrionReplicas {
		deployment.Spec.Replicas = &environment.Spec.OrionReplicas

		logger.Info("scaling orion server", "replicas", environment.Spec.OrionReplicas, "name", deploymentName)

		if err := r.Update(ctx, deployment); err != nil {
			return err
		}
	}

	return nil
}

func (r *PrefectEnvironmentReconciler) reconcileOrionServerService(ctx context.Context, environment *mlopsv1alpha1.PrefectEnvironment) error {
	logger := log.FromContext(ctx)

	serviceLabels := map[string]string{
		"mlops.aigency.com/environment": environment.Name,
		"mlops.aigency.com/component":   "orion-server",
	}

	serviceName := fmt.Sprintf("%s-orion-server", environment.Name)

	service := &corev1.Service{}

	if err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: environment.GetNamespace()}, service); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("creating orion server service", "name", environment.GetName())

			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: environment.GetNamespace(),
					Labels:    serviceLabels,
				},
				Spec: corev1.ServiceSpec{
					Type:     corev1.ServiceTypeClusterIP,
					Selector: serviceLabels,
					Ports: []corev1.ServicePort{
						{
							TargetPort: intstr.FromInt(4200),
							Port:       4200,
							Name:       "http-orion",
						},
					},
				},
			}

			if err := ctrl.SetControllerReference(environment, service, r.Scheme); err != nil {
				return err
			}

			if err := r.Create(ctx, service); err != nil {
				return err
			}

			return nil
		}

		return err
	}

	return nil
}
