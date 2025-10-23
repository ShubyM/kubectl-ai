#!/usr/bin/env bash
# Create namespace and a deployment with an invalid image that will cause ImagePullBackOff
kubectl delete namespace deployment --ignore-not-found
kubectl create namespace deployment
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:v1.1.26-patch # This will cause ImagePullBackOff error
EOF

# Wait for deployment's pod to enter ImagePullBackOff state
for i in {1..30}; do
    if kubectl get pods -n deployment -l app=nginx -o jsonpath='{.items[0].status.containerStatuses[0].state.waiting.reason}' | grep -q "ImagePullBackOff"; then
        exit 0
    fi
    sleep 1
done 