#!/usr/bin/env bash
set -e

NAMESPACE="game-backend"
K8S_DIR="../k8s"

kubectl delete -f "$K8S_DIR" -n "$NAMESPACE" --ignore-not-found

kubectl delete namespace "$NAMESPACE" --ignore-not-found

kubectl delete -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml --ignore-not-found

kind delete cluster


