#!/usr/bin/env bash
set -e

NAMESPACE="game-backend"
K8S_DIR="../k8s"

kubectl delete -f "$K8S_DIR" -n "$NAMESPACE" --ignore-not-found

kubectl delete namespace "$NAMESPACE" --ignore-not-found

minikube image rm "game-backend_matchmaking:latest"
minikube image rm "game-backend_server:latest"

minikube delete


