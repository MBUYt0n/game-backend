#!/usr/bin/env bash
set -e

NAMESPACE="game-backend"
IMAGE="game-backend_matchmaking:latest"
K8S_DIR="../k8s"
IMAGE2="game-backend_server:latest"

kind create cluster --config kind-config.yaml

kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml 

until kubectl get pods -n ingress-nginx \
  -l app.kubernetes.io/component=controller \
  --no-headers 2>/dev/null | grep -q .; do
  echo "Waiting for ingress-nginx controller pods to be created..."
  sleep 2
done

kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=120s

until kubectl get endpoints ingress-nginx-controller-admission \
  -n ingress-nginx \
  -o jsonpath='{.subsets[0].addresses[0].ip}' 2>/dev/null | grep -q .; do
  echo "Waiting for ingress-nginx controller admission endpoint to be ready..."
  sleep 2
done

kubectl create namespace "$NAMESPACE"

kind load docker-image "$IMAGE"

kind load docker-image "$IMAGE2"

kubectl apply -f "$K8S_DIR" -n "$NAMESPACE"

kubectl rollout status deployment matchmaking -n "$NAMESPACE"

kubectl get pods -n "$NAMESPACE"
