package serving

import (
	kservev1alpha1 "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	kservev1beta1 "github.com/kserve/kserve/pkg/apis/serving/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8srbacv1 "k8s.io/api/rbac/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	testutils "github.com/opendatahub-io/odh-model-controller/test/utils"
)

const (
	scopedRoleName        = "odh-model-controller-scoped-access"
	scopedRoleBindingName = "odh-model-controller-scoped-access-binding"
)

var _ = Describe("RBACScope Integration", func() {

	Describe("InferenceService reconciliation creates scoped RBAC", func() {
		var testNs string

		BeforeEach(func() {
			testNs = testutils.Namespaces.Create(ctx, k8sClient).Name
		})

		It("should create a scoped Role and RoleBinding when an InferenceService is created", func() {
			By("Creating a ServingRuntime")
			sr := &kservev1alpha1.ServingRuntime{}
			Expect(testutils.ConvertToStructuredResource(KserveServingRuntimePath1, sr)).NotTo(HaveOccurred())
			sr.SetNamespace(testNs)
			Expect(k8sClient.Create(ctx, sr)).Should(Succeed())

			By("Creating an InferenceService")
			isvc := &kservev1beta1.InferenceService{}
			Expect(testutils.ConvertToStructuredResource(KserveInferenceServicePath1, isvc)).NotTo(HaveOccurred())
			isvc.SetNamespace(testNs)
			Expect(k8sClient.Create(ctx, isvc)).Should(Succeed())

			By("Verifying the scoped Role is created")
			role := &k8srbacv1.Role{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      scopedRoleName,
					Namespace: testNs,
				}, role)
			}, timeout, interval).Should(Succeed())

			Expect(role.Rules).NotTo(BeEmpty())
			// Verify the rules contain the expected resources
			resourceSet := map[string]bool{}
			for _, rule := range role.Rules {
				for _, res := range rule.Resources {
					resourceSet[res] = true
				}
			}
			Expect(resourceSet).To(HaveKey("secrets"))
			Expect(resourceSet).To(HaveKey("configmaps"))
			Expect(resourceSet).To(HaveKey("services"))
			Expect(resourceSet).To(HaveKey("serviceaccounts"))

			By("Verifying the scoped RoleBinding is created")
			rb := &k8srbacv1.RoleBinding{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      scopedRoleBindingName,
					Namespace: testNs,
				}, rb)
			}, timeout, interval).Should(Succeed())

			Expect(rb.RoleRef.Kind).To(Equal("Role"))
			Expect(rb.RoleRef.Name).To(Equal(scopedRoleName))
			Expect(rb.Subjects).To(HaveLen(1))
			Expect(rb.Subjects[0].Kind).To(Equal("ServiceAccount"))
			Expect(rb.Subjects[0].Name).To(Equal("odh-model-controller"))
		})

		It("should clean up scoped RBAC when the last InferenceService is deleted", func() {
			By("Creating a ServingRuntime and InferenceService")
			sr := &kservev1alpha1.ServingRuntime{}
			Expect(testutils.ConvertToStructuredResource(KserveServingRuntimePath1, sr)).NotTo(HaveOccurred())
			sr.SetNamespace(testNs)
			Expect(k8sClient.Create(ctx, sr)).Should(Succeed())

			isvc := &kservev1beta1.InferenceService{}
			Expect(testutils.ConvertToStructuredResource(KserveInferenceServicePath1, isvc)).NotTo(HaveOccurred())
			isvc.SetNamespace(testNs)
			Expect(k8sClient.Create(ctx, isvc)).Should(Succeed())

			By("Waiting for scoped Role to be created")
			role := &k8srbacv1.Role{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      scopedRoleName,
					Namespace: testNs,
				}, role)
			}, timeout, interval).Should(Succeed())

			By("Deleting the InferenceService")
			Expect(k8sClient.Delete(ctx, isvc)).Should(Succeed())

			By("Verifying the scoped Role is cleaned up")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      scopedRoleName,
					Namespace: testNs,
				}, &k8srbacv1.Role{})
				return k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			By("Verifying the scoped RoleBinding is cleaned up")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      scopedRoleBindingName,
					Namespace: testNs,
				}, &k8srbacv1.RoleBinding{})
				return k8sErrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})
})
