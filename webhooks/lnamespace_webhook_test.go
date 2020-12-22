package webhooks_test

import (
	"context"
	"encoding/json"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	"github.com/loblaw-sre/namespace-controller/webhooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	// +kubebuilder:scaffold:imports
)

var _ = Describe("webhook", func() {
	var k8sClient client.Client
	var ns *gialv1beta1.LNamespace
	var lnd *webhooks.LNamespaceDefaulter
	BeforeEach(func(done Done) {
		k8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		lnd = &webhooks.LNamespaceDefaulter{
			Client:               k8sClient,
			DefaultIstioRevision: "istio-version-1",
		}
		lnd.InjectDecoder(decoder)
		ns = &gialv1beta1.LNamespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: DefaultName,
			},
		}
		close(done)
	}, TestTimeout)
	When("the request is submitted", func() {
		var res admission.Response
		BeforeEach(func(done Done) {
			raw, err := json.Marshal(ns)
			Expect(err).ToNot(HaveOccurred(), "Marshalling namespace definition should not have errored.")
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: raw,
					},
					UserInfo: authenticationv1.UserInfo{
						Username: john,
					},
				},
			}
			ctx := context.Background()
			res = lnd.Handle(ctx, req)
			Expect(res.Allowed).To(BeTrue(), "Resource should be accepted by the admission webhook.")
			close(done)
		}, TestTimeout)
		It("defaults the revision to the default istio version", func(done Done) {
			Expect(res.Patches).To(ContainElement(
				MatchFields(IgnoreExtras, Fields{
					"Operation": Equal("add"),
					"Path":      Equal("/spec/istioRevision"),
					"Value":     BeEquivalentTo(lnd.DefaultIstioRevision),
				}),
			))
			close(done)
		})
		It("defaults the sudoers list to the requester, if not provided", func(done Done) {
			Expect(res.Patches).To(ContainElement(
				MatchFields(IgnoreExtras, Fields{
					"Operation": Equal("add"),
					"Path":      Equal("/spec/sudoers"),
					"Value": ConsistOf(
						HaveKeyWithValue(
							"name", BeEquivalentTo(john),
						),
					),
				}),
			))
			close(done)
		}, TestTimeout)
	})
})
