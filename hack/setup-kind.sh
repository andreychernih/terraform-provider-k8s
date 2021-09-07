#!/usr/bin/env bash

# Dependencies
#  - kind >=0.11
#  - k8s >= 1.19

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

METALLB_VERSION=v0.10.2
INGRESS_VERSION=controller-v1.0.0

# Fingers crossed this is at least a /24 range
METALLB_IP_PREFIX_RANGE=$(docker network inspect kind --format '{{(index .IPAM.Config 0).Subnet}}' | sed -r 's/(.*).\/.*/\1/')
METALLB_IP_ADDRESS_RANGE=$(echo "${METALLB_IP_PREFIX_RANGE}200-${METALLB_IP_PREFIX_RANGE}250" | sed "s/\./\\\./g")

kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/${METALLB_VERSION}/manifests/namespace.yaml
kubectl delete secret -n metallb-system memberlist --ignore-not-found=true
kubectl create secret generic -n metallb-system memberlist --from-literal=secretkey="$(openssl rand -base64 128)"
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/${METALLB_VERSION}/manifests/metallb.yaml

kubectl wait --namespace metallb-system --for=condition=ready pod --selector=app=metallb --timeout=90s

sed "s/METALLB_IP_ADDRESS_RANGE/${METALLB_IP_ADDRESS_RANGE}/" "${SCRIPT_DIR}/metallb-configmap.yaml" | kubectl apply -f -

kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/${INGRESS_VERSION}/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=90s

# Patch the ingress class to make it defaul
kubectl patch ingressclass nginx -p '{"metadata": {"annotations":{"ingressclass.kubernetes.io/is-default-class": "true"}}}'
