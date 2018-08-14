#!/bin/bash

kubectl logs -n anthill -f `kubectl get po -n anthill -l anthill=operator -o template --template='{{index .items 0 "metadata" "name"}}'`
