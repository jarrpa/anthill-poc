#!/bin/bash

kubectl delete -n anthill -f Anthill.yaml
kubectl delete -n anthill -f AnthillController.yaml
