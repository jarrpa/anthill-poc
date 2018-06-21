#! /bin/bash

kubectl exec -it -n anthill `kubectl get po -n anthill -l demo=heketi-node -o template --template '{{index .items 0 "metadata" "name"}}'` heketi-cli $@
