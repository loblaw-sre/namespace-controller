package controllers_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	"github.com/loblaw-sre/namespace-controller/controllers"
	"github.com/loblaw-sre/namespace-controller/pkg/utils"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

// used to dynamically generate test cases for ClusterRole and ClusterRoleBinding
type checkExistenceStruct struct {
	user   string //cluster role and cluster role binding with this username
	exists bool   //should or should not exist.
}

// used to intelligently determine resourceNotFound
type resourceState struct {
	resource client.Object
	err      error
}

// generate test cases for both cluster roles and cluster role bindings based on checkExistenceStructs
var _ = Describe("RBAC Controller", func() {
	var k8sClient client.Client
	var rbacr *controllers.RBACReconciler
	var ctx context.Context
	var nsList []*gialv1beta1.LNamespace

	// takes a list of checkExistences and runs them through the ExistAsAResource matcher as an individual test case
	var generateSelfImpersonatorTests = func(existences []checkExistenceStruct) {
		for _, v := range existences {
			v := v
			e := ""
			if !v.exists {
				e = "no longer "
			}
			for _, resource := range []client.Object{&rbacv1.ClusterRole{}, &rbacv1.ClusterRoleBinding{}} {
				r := resource
				gvk, err := apiutil.GVKForObject(r, scheme.Scheme)
				if err != nil {
					panic("Error while generating test cases.")
				}
				It(fmt.Sprintf("cluster %scontains a %s for %s", e, gvk.Kind, v.user), func(done Done) {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name: utils.Slug(v.user) + "-impersonator",
					}, r)
					Expect(resourceState{resource: r, err: err}).Should(ExistAsAResource(v.exists))
					close(done)
				}, TestTimeout)
			}
		}
	}

	BeforeEach(func(done Done) {
		k8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
		rbacr = &controllers.RBACReconciler{
			Client:   k8sClient,
			Log:      logf.Log,
			Recorder: record.NewFakeRecorder(64),
		}
		ctx = context.Background()
		close(done)
	}, TestTimeout)

	Context("One namespace", func() {
		var ns *gialv1beta1.LNamespace
		BeforeEach(func(done Done) {
			nsList = []*gialv1beta1.LNamespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: DefaultName,
					},
					Spec: gialv1beta1.LNamespaceSpec{
						Sudoers: []rbacv1.Subject{
							{Name: john, Kind: "User"},
						},
						Developers: []rbacv1.Subject{
							{Name: bob, Kind: "User"},
						},
						Managers: []rbacv1.Subject{
							{Name: alice, Kind: "User"},
						},
					},
				},
			}
			ns = nsList[0]
			Expect(k8sClient.Create(ctx, nsList[0])).ToNot(HaveOccurred(), "Creating namespace %s should not have errored.", ns.Name)
			_, err := rbacr.Reconcile(ctx, controllerruntime.Request{NamespacedName: types.NamespacedName{Name: ns.Name}})
			Expect(err).ToNot(HaveOccurred(), "Reconcile should not have errored.")
			close(done)
		}, TestTimeout)
		Context("metadata", func() {
			//TODO: how can we test the garbage collector logic re: orphaning? We don't actually run it in our test env. This is worth running a test env for.
			Context("self-impersonator resources should contain the self-impersonator label", func() {
				It("self-impersonator cluster role for john", func(done Done) {
					cr := &rbacv1.ClusterRole{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{
						Name: utils.Slug(john) + "-impersonator",
					}, cr)).ToNot(HaveOccurred())
					Expect(cr.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelSelfImpersonator))
					close(done)
				}, TestTimeout)
				It("self-impersonator cluster role binding for john", func(done Done) {
					crb := &rbacv1.ClusterRoleBinding{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{
						Name: utils.Slug(john) + "-impersonator",
					}, crb)).ToNot(HaveOccurred())
					Expect(crb.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelSelfImpersonator))
					close(done)
				}, TestTimeout)
			})
			Context("sudoer group impersonator resources should contain the sudoer group impersonator label", func() {
				It("sudoer group impersonator CR for lns", func(done Done) {
					cr := &rbacv1.ClusterRole{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns.GetSudoersGroupName()}, cr)).ToNot(HaveOccurred())
					Expect(cr.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelSudoerImpersonator))
					close(done)
				}, TestTimeout)
				It("sudoer group impersonator CRB for lns", func(done Done) {
					crb := &rbacv1.ClusterRoleBinding{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns.GetSudoersGroupName()}, crb)).ToNot(HaveOccurred())
					Expect(crb.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelSudoerImpersonator))
					close(done)
				}, TestTimeout)
			})
			Context("sudoer permissions resources should contain the sudoer permissions label", func() {

				It("Namespace editor CRB", func(done Done) {
					crb := &rbacv1.ClusterRoleBinding{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name + "-sudoeditor"}, crb)).ToNot(HaveOccurred())
					Expect(crb.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelSudoerPermissions))
					close(done)
				}, TestTimeout)
				It("namespace cluster-admin RB", func(done Done) {
					rb := &rbacv1.RoleBinding{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns.GetSudoersGroupName(), Namespace: ns.Name}, rb)).ToNot(HaveOccurred())
					Expect(rb.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelSudoerPermissions))
					close(done)
				}, TestTimeout)
			})

			Context("manager resources should contain the manager label", func() {
				It("LNamespace editor CR", func(done Done) {
					cr := &rbacv1.ClusterRole{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name + "-editor"}, cr)).ToNot(HaveOccurred())
					Expect(cr.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelManagerPermissions))
					close(done)
				}, TestTimeout)
				It("LNamespace editor CRB", func(done Done) {
					crb := &rbacv1.ClusterRoleBinding{}
					Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ns.Name + "-manager"}, crb)).ToNot(HaveOccurred())
					Expect(crb.Labels).To(HaveKeyWithValue(controllers.LabelKey, controllers.LabelManagerPermissions))

					close(done)
				}, TestTimeout)
			})
		})
		Context("self impersonators", func() {
			generateSelfImpersonatorTests([]checkExistenceStruct{{user: john, exists: true}})
		})
		Context("self impersonator cleanup", func() {
			BeforeEach(func(done Done) {
				ns.Spec.Sudoers = []rbacv1.Subject{
					{Name: john, Kind: "User"},
				}
				Expect(k8sClient.Update(ctx, ns)).ToNot(HaveOccurred(), "Updating namespace %s should not have errored.", ns.Name)
				_, err := rbacr.Reconcile(ctx, controllerruntime.Request{NamespacedName: types.NamespacedName{Name: ns.Name}})
				Expect(err).ToNot(HaveOccurred(), "Reconcile should not have errored.")
				close(done)
			}, TestTimeout)
			generateSelfImpersonatorTests([]checkExistenceStruct{{user: alice, exists: false}})
		})

		// generate sudoer group rbac tests
		Context("sudoer group ClusterRole", func() {
			c := &rbacv1.ClusterRole{}
			BeforeEach(func(done Done) {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: ns.GetSudoersGroupName(),
				}, c)
				Expect(resourceState{resource: c, err: err}).To(ExistAsAResource(true))
				close(done)
			}, TestTimeout)

			It("contains the ability to impersonate the namespace", func(done Done) {
				Expect(c.Rules).To(HaveLen(1))
				Expect(c.Rules[0].ResourceNames).To(Equal([]string{ns.GetSudoersGroupName()}))
				Expect(c.Rules[0].Verbs).To(Equal([]string{"impersonate"}))
				Expect(c.Rules[0].Resources).To(Equal([]string{"groups"}))
				close(done)
			}, TestTimeout)
		})
		Context("sudoer group ClusterRoleBinding", func() {
			c := &rbacv1.ClusterRoleBinding{}
			BeforeEach(func(done Done) {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: ns.GetSudoersGroupName(),
				}, c)
				Expect(resourceState{resource: c, err: err}).To(ExistAsAResource(true))
				close(done)
			}, TestTimeout)
			It("contains all of the sudoer members", func(done Done) {
				Expect(c.Subjects).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal(john),
						"Kind": Equal("User"),
					}),
				))
				close(done)
			}, TestTimeout)
			It("refers to the sudoer ClusterRole", func(done Done) {
				Expect(c.RoleRef.Name).To(Equal(ns.GetSudoersGroupName()))
				Expect(c.RoleRef.Kind).To(Equal("ClusterRole"))
				close(done)
			}, TestTimeout)
		})

		Context("check sudoer permissions", func() {
			It("allows a sudoer to edit the namespace", func(done Done) {
				var allRules []rbacv1.PolicyRule
				crbl := &rbacv1.ClusterRoleBindingList{}
				Expect(k8sClient.List(ctx, crbl)).ToNot(HaveOccurred())
				for _, crb := range crbl.Items {
					for _, s := range crb.Subjects {
						if s.Name == ns.GetSudoersGroupName() {
							cr := &rbacv1.ClusterRole{}
							Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crb.RoleRef.Name}, cr)).ToNot(HaveOccurred())
							allRules = append(allRules, cr.Rules...)
						}
					}
				}
				Expect(allRules).To(ContainElement(
					MatchFields(IgnoreExtras, Fields{
						"ResourceNames": ContainElements(ns.Name),
						"Verbs":         ContainElements("update", "patch"),
					}),
				))
				close(done)
			}, TestTimeout)
			It("contains cluster-admin privileges inside the namespace", func(done Done) {
				var found bool
				rbl := &rbacv1.RoleBindingList{}
				Expect(k8sClient.List(ctx, rbl, &client.ListOptions{
					Namespace: ns.Name,
				})).ToNot(HaveOccurred())
				for _, rb := range rbl.Items {
					for _, s := range rb.Subjects {
						if s.Name == ns.GetSudoersGroupName() && rb.RoleRef.Name == "cluster-admin" && rb.RoleRef.Kind == "ClusterRole" {
							found = true
						}
					}
				}
				Expect(found).To(BeTrue(), "cluster-admin was not found")
				close(done)
			}, TestTimeout)
		})
		Context("user permissions", func() {
			It("has a permission for bob to use user permissions", func(done Done) {
				var found bool
				rbl := &rbacv1.RoleBindingList{}
				Expect(k8sClient.List(ctx, rbl, &client.ListOptions{Namespace: ns.Name})).ToNot(HaveOccurred())
				for _, v := range rbl.Items {
					for _, s := range v.Subjects {
						if s.Name == bob && v.RoleRef.Name == "admin" {
							found = true
						}
					}
				}
				Expect(found).To(BeTrue())
				close(done)
			}, TestTimeout)
		})
		Context("manager permissions", func() {
			It("alice can edit lnamespace resources that she owns.", func(done Done) {
				var found bool
				crbl := &rbacv1.ClusterRoleBindingList{}
				Expect(k8sClient.List(ctx, crbl)).ToNot(HaveOccurred())
				for _, v := range crbl.Items {
					for _, s := range v.Subjects {
						if s.Name == alice && v.RoleRef.Name == ns.Name+"-editor" {
							found = true
						}
					}
				}
				Expect(found).To(BeTrue())
				close(done)
			}, TestTimeout)
		})
	})

	Context("Two namespaces", func() {
		BeforeEach(func(done Done) {
			nsList = []*gialv1beta1.LNamespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: DefaultName + "-1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: DefaultName + "-2",
					},
				},
			}
			close(done)
		}, TestTimeout)

		Context("self-impersonator RBAC cleanup logic", func() {
			BeforeEach(func(done Done) {
				for k := range nsList {
					nsList[k].Spec.Sudoers = []rbacv1.Subject{
						{
							Name: john,
							Kind: "User",
						},
					}
					Expect(k8sClient.Create(ctx, nsList[k])).ToNot(HaveOccurred(), "Creating namespace %s should not have errored.", nsList[k].Name)
					_, err := rbacr.Reconcile(ctx, controllerruntime.Request{NamespacedName: types.NamespacedName{Name: nsList[k].Name}})
					Expect(err).ToNot(HaveOccurred(), "Reconcile should not have errored.")
				}
				close(done)
			}, TestTimeout)
			When("one namespace changes sudoer", func() {
				BeforeEach(func(done Done) {
					nsList[0].Spec.Sudoers = []rbacv1.Subject{
						{
							Name: alice,
							Kind: "User",
						},
					}
					Expect(k8sClient.Update(ctx, nsList[0])).ToNot(HaveOccurred(), "Updating namespace %s should not have errored.", nsList[0].Name)
					_, err := rbacr.Reconcile(ctx, controllerruntime.Request{NamespacedName: types.NamespacedName{Name: nsList[0].Name}})
					Expect(err).ToNot(HaveOccurred(), "Reconcile should not have errored.")
					close(done)
				}, TestTimeout)
				generateSelfImpersonatorTests([]checkExistenceStruct{
					{user: john, exists: true},
					{user: alice, exists: true},
				})

				When("the other namespace changes sudoer", func() {
					BeforeEach(func(done Done) {
						nsList[1].Spec.Sudoers = []rbacv1.Subject{
							{
								Name: bob,
								Kind: "User",
							},
						}
						Expect(k8sClient.Update(ctx, nsList[1])).ToNot(HaveOccurred(), "Updating namespace %s should not have errored.", nsList[1].Name)
						_, err := rbacr.Reconcile(ctx, controllerruntime.Request{NamespacedName: types.NamespacedName{Name: nsList[1].Name}})
						Expect(err).ToNot(HaveOccurred(), "Reconcile should not have errored.")
						close(done)
					}, TestTimeout)
					generateSelfImpersonatorTests([]checkExistenceStruct{
						{user: john, exists: false},
						{user: alice, exists: true},
						{user: bob, exists: true},
					})
				})
			})
		})
	})

})
