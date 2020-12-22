/*
Copyright 2020.

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

package controllers_test

import (
	"errors"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

const (
	StartupTimeout    = 30
	TestTimeout       = 10
	EventuallyTimeout = 8 //Eventually calls should time out before the end of the test

	DefaultName = "test-ns"
	john        = "john@loblaw.ca"
	alice       = "alice@loblaw.ca"
	bob         = "bob@loblaw.ca"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	err := gialv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	// +kubebuilder:scaffold:scheme

	close(done)
}, StartupTimeout)

type resourceExistsMatcher struct {
	exists bool
}

func (m *resourceExistsMatcher) Match(actual interface{}) (success bool, err error) {
	resourceState, ok := actual.(resourceState)
	if !ok {
		return false, errors.New("did not receive resourceState")
	}

	if m.exists && resourceState.err == nil {
		return true, nil
	}
	if !m.exists && apierrors.IsNotFound(resourceState.err) {
		return true, nil
	}

	// if there is a valid UID, then resource exists. Else, it would've been
	// cleaned up by the GC.
	var validUID bool
	for _, v := range resourceState.resource.GetOwnerReferences() {
		if v.UID != "mark-for-deletion" {
			validUID = true
		}
	}
	return validUID == m.exists, nil
}

func (m *resourceExistsMatcher) FailureMessage(actual interface{}) (message string) {
	not := ""
	if !m.exists {
		not = "not "
	}
	return fmt.Sprintf("Expected\n\t%#v\n to result in %sfound", actual, not)
}

func (m *resourceExistsMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	not := ""
	if !m.exists {
		not = "not "
	}
	return fmt.Sprintf("Expected\n\t%#v\n to not result in %sfound", actual, not)
}

func ExistAsAResource(expected interface{}) types.GomegaMatcher {
	return &resourceExistsMatcher{
		exists: expected.(bool),
	}
}
