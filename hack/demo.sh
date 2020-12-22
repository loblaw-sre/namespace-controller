#! /bin/bash

make up
hack/new-user.sh

kubectl get lns
# show namespace-no-sudoer.yaml
kubectl apply -f config/samples/namespace-no-sudoer.yaml
kubectl get lns

kubens namespace-sample
kubectl get po

# Fail
kubectl run temp --image=byrnedo/alpine-curl --rm -it --restart=Never --command -- sh
# Pass
kubectl --as john@example.com --as-group namespace-sample-sudoers run temp --image=byrnedo/alpine-curl --rm -it --restart=Never --command -- sh


kubectl apply -f config/samples/namespace-with-other-sudoer.yaml
kubectl edit lns namespace-sample-2
kubectl --as=john@example.com --as-group=namespace-sample-2-sudoers edit lns namespace-sample-2
