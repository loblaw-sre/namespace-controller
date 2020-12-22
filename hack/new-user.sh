#! /bin/bash

cd $(mktemp -d)
pwd
openssl genrsa -out new-user.key 2048
openssl req -new -key new-user.key -out new-user.csr -subj "/O=system:authenticated/CN=john@example.com"
jq -r ".spec.request = \"$(cat new-user.csr | base64 -w 0)\"" <<EOF | kubectl apply -f -
{
  "apiVersion": "certificates.k8s.io/v1",
  "kind": "CertificateSigningRequest",
  "metadata": {
    "name": "john"
  },
  "spec": {
    "groups": [
      "system:authenticated"
    ],
    "request": "test",
    "signerName": "kubernetes.io/kube-apiserver-client",
    "usages": [
      "client auth"
    ]
  }
}
EOF
kubectl certificate approve john
kubectl get csr john -o json | jq -r .status.certificate | base64 -d > new-user.crt
kubectl config set-credentials kind-namespace-controller-john --client-key=${PWD}/new-user.key --client-certificate=${PWD}/new-user.crt --embed-certs=true
kubectl config set-context kind-namespace-controller-john --cluster=kind-namespace-controller --user=kind-namespace-controller-john
kubectx kind-namespace-controller-john
