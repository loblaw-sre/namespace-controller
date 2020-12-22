package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	"github.com/loblaw-sre/namespace-controller/controllers"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespace Controller", func() {
	var ctx context.Context
	var ns *gialv1beta1.LNamespace
	var nsr *controllers.NamespaceReconciler
	var k8sClient client.Client

	BeforeEach(func(done Done) {
		k8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		nsr = &controllers.NamespaceReconciler{
			Client:   k8sClient,
			Log:      logf.Log,
			Recorder: record.NewFakeRecorder(64),
		}
		ns = &gialv1beta1.LNamespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: DefaultName,
			},
			Spec: gialv1beta1.LNamespaceSpec{
				IstioRevision: "istio-version-1",
				Billing: map[string]string{
					"budget": "1.0",
				},
			},
		}
		ctx = context.Background()
		close(done)
	}, TestTimeout)
	Describe("namespace creation", func() {
		var rawNs *corev1.Namespace

		JustBeforeEach(func(done Done) {
			err := k8sClient.Create(ctx, ns)
			Expect(err).ToNot(HaveOccurred(), "Creating LNamespace should not have errored.")
			_, err = nsr.Reconcile(ctx, controllerruntime.Request{
				NamespacedName: types.NamespacedName{
					Name: ns.Name,
				},
			})
			Expect(err).ToNot(HaveOccurred(), "Reconciling LNamespace should not have errored.")
			rawNs = &corev1.Namespace{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name: ns.Name,
			}, rawNs)
			Expect(err).ShouldNot(HaveOccurred(), "Getting raw namespace should not have errored.")
			close(done)
		})

		Context("basic namespace", func() {
			It("has the correct billing annotations", func(done Done) {
				Expect(rawNs.Annotations["budget"]).To(Equal("1.0"))
				close(done)
			}, TestTimeout)
			It("has its istio revision injected into the core namespace", func(done Done) {
				Expect(rawNs.Labels[controllers.IstioTag]).To(Equal("istio-version-1"))
				close(done)
			}, TestTimeout)
		})

		Context("with namespace label overrides", func() {
			BeforeEach(func(done Done) {
				ns.Spec.NamespaceLabelOverrides = map[string]string{
					controllers.IstioTag: "istio-version-override",
					"custom":             "this-is-a-custom-value",
				}
				close(done)
			}, TestTimeout)

			It("custom labels are set", func(done Done) {
				Expect(rawNs.Labels["custom"]).Should(Equal("this-is-a-custom-value"))
				close(done)
			})

			It("has its istio revision overriden in the namespace labels", func(done Done) {
				Expect(rawNs.Labels[controllers.IstioTag]).To(Equal("istio-version-override"))
				close(done)
			}, TestTimeout)
		})
	})
})
