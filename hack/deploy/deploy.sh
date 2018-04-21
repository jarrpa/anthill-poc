#! /bin/bash

kubectl create -f AnthillController.yaml
while [[ "$(kubectl get po -n anthill -l anthill=controller -o template --template '{{index .items 0 "status" "conditions" 1 "status"}}')" != "True" ]]; do
  sleep 1
done
kubectl apply -f Anthill.yaml
kubectl logs -n anthill -f `kubectl get po -n anthill -l anthill=controller -o template --template '{{index .items 0 "metadata" "name"}}'`
