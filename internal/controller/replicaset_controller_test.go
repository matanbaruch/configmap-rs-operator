package controller

import (
	"context"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/matanbaruch/configmap-rs-operator/internal/config"
)

var _ = ginkgo.Describe("ReplicaSetController", func() {
	var (
		ctx        context.Context
		reconciler *ReplicaSetReconciler
		fakeClient client.Client
		testConfig *config.OperatorConfig
	)

	ginkgo.BeforeEach(func() {
		ctx = context.Background()
		testConfig = &config.OperatorConfig{
			NamespaceRegex: []string{},
			DryRun:         false,
			Debug:          true,
			Trace:          false,
		}

		s := runtime.NewScheme()
		_ = scheme.AddToScheme(s)
		fakeClient = fake.NewClientBuilder().WithScheme(s).Build()

		reconciler = &ReplicaSetReconciler{
			Client: fakeClient,
			Scheme: s,
			Config: testConfig,
		}
	})

	ginkgo.Context("When reconciling a ReplicaSet", func() {
		ginkgo.It("Should add owner reference to ConfigMap when ReplicaSet mounts it", func() {
			// Create a ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			gomega.Expect(fakeClient.Create(ctx, configMap)).To(gomega.Succeed())

			// Create a ReplicaSet that mounts the ConfigMap
			replicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rs",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: appsv1.ReplicaSetSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-image",
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "config-vol",
											MountPath: "/etc/config",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config-vol",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			gomega.Expect(fakeClient.Create(ctx, replicaSet)).To(gomega.Succeed())

			// Reconcile
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rs",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(reconcile.Result{}))

			// Verify owner reference was added
			var updatedConfigMap corev1.ConfigMap
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-config",
				Namespace: "default",
			}, &updatedConfigMap)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(updatedConfigMap.OwnerReferences).To(gomega.HaveLen(1))
			gomega.Expect(updatedConfigMap.OwnerReferences[0].Name).To(gomega.Equal("test-rs"))
			gomega.Expect(updatedConfigMap.OwnerReferences[0].Kind).To(gomega.Equal("ReplicaSet"))
		})

		ginkgo.It("Should not add owner reference when ConfigMap doesn't exist", func() {
			// Create a ReplicaSet that mounts a non-existent ConfigMap
			replicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rs",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: appsv1.ReplicaSetSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-image",
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "config-vol",
											MountPath: "/etc/config",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config-vol",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "non-existent-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			gomega.Expect(fakeClient.Create(ctx, replicaSet)).To(gomega.Succeed())

			// Reconcile
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rs",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(reconcile.Result{}))
		})

		ginkgo.It("Should skip ReplicaSet when namespace doesn't match regex", func() {
			testConfig.NamespaceRegex = []string{"^kube-.*"}

			// Create a ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default", // This won't match the regex
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			gomega.Expect(fakeClient.Create(ctx, configMap)).To(gomega.Succeed())

			// Create a ReplicaSet
			replicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rs",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: appsv1.ReplicaSetSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-image",
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "config-vol",
											MountPath: "/etc/config",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config-vol",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			gomega.Expect(fakeClient.Create(ctx, replicaSet)).To(gomega.Succeed())

			// Reconcile
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rs",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(reconcile.Result{}))

			// Verify owner reference was NOT added
			var updatedConfigMap corev1.ConfigMap
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-config",
				Namespace: "default",
			}, &updatedConfigMap)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(updatedConfigMap.OwnerReferences).To(gomega.BeEmpty())
		})

		ginkgo.It("Should skip changes in dry-run mode", func() {
			testConfig.DryRun = true

			// Create a ConfigMap
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			gomega.Expect(fakeClient.Create(ctx, configMap)).To(gomega.Succeed())

			// Create a ReplicaSet
			replicaSet := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rs",
					Namespace: "default",
					UID:       "test-uid",
				},
				Spec: appsv1.ReplicaSetSpec{
					Replicas: int32Ptr(1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test-container",
									Image: "test-image",
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "config-vol",
											MountPath: "/etc/config",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "config-vol",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-config",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			gomega.Expect(fakeClient.Create(ctx, replicaSet)).To(gomega.Succeed())

			// Reconcile
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-rs",
					Namespace: "default",
				},
			}

			result, err := reconciler.Reconcile(ctx, req)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(result).To(gomega.Equal(reconcile.Result{}))

			// Verify owner reference was NOT added due to dry-run
			var updatedConfigMap corev1.ConfigMap
			err = fakeClient.Get(ctx, types.NamespacedName{
				Name:      "test-config",
				Namespace: "default",
			}, &updatedConfigMap)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(updatedConfigMap.OwnerReferences).To(gomega.BeEmpty())
		})
	})
})

func TestReplicaSetController(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "ReplicaSetController Suite")
}

func int32Ptr(i int32) *int32 {
	return &i
}
