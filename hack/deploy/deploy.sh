#! /bin/bash

kubectl create -f Anthill.yaml
while [[ "$(kubectl get po -n anthill -l anthill=operator -o template --template '{{index .items 0 "status" "conditions" 1 "status"}}')" != "True" ]]; do
  sleep 1
done

kubectl apply -f GlusterCluster-1.yaml

./tail.sh
