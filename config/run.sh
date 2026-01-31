#!/bin/bash
NAMESPACE="game-backend"
IMAGE="game-backend_matchmaking:latest"
K8S_DIR="../k8s"
IMAGE2="game-backend_server:latest"

minikube start

minikube addons enable ingress

kubectl wait \
  --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=180s


kubectl create namespace "$NAMESPACE"

minikube image load "$IMAGE"

minikube image load "$IMAGE2"

kubectl apply -f "$K8S_DIR" -n "$NAMESPACE"

kubectl rollout status deployment matchmaking -n "$NAMESPACE"

kubectl get pods -n "$NAMESPACE"
