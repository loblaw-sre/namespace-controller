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

package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	gialv1beta1 "github.com/loblaw-sre/namespace-controller/api/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-gial-lblw-dev-v1beta1-lnamespace,mutating=true,failurePolicy=fail,sideEffects=None,groups=gial.lblw.dev,resources=lnamespaces,verbs=create;update,versions=v1beta1,name=mlnamespace.kb.io,admissionReviewVersions={v1,v1beta1}

type LNamespaceDefaulter struct {
	Client               client.Client
	DefaultIstioRevision string
	decoder              *admission.Decoder
}

var _ admission.Handler = &LNamespaceDefaulter{}

//Handle implements admission.Handlerp
func (lnd *LNamespaceDefaulter) Handle(ctx context.Context, req admission.Request) admission.Response {
	ns := &gialv1beta1.LNamespace{}
	err := lnd.decoder.Decode(req, ns)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.UserInfo.Username == "" {
		return admission.Errored(http.StatusBadRequest, errors.New("requesting user cannot be empty"))
	}
	if ns.Spec.Sudoers == nil {
		ns.Spec.Sudoers = []rbacv1.Subject{
			{
				Name: req.UserInfo.Username,
				Kind: "User",
			},
		}
	}
	if ns.Spec.IstioRevision == "" {
		ns.Spec.IstioRevision = lnd.DefaultIstioRevision
	}
	marshalledNS, err := json.Marshal(ns)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshalledNS)
}

// InjectDecoder implements "sigs.k8s.io/controller-runtime/pkg/webhook/admission".DecoderInjector
func (lnd *LNamespaceDefaulter) InjectDecoder(d *admission.Decoder) error {
	lnd.decoder = d
	return nil
}
