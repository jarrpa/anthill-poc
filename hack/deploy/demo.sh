#!/bin/bash

ip=`kubectl get ep -n anthill demo -o template --template '{{range index .subsets 0 "addresses"}}{{if eq .hostname "demo-heketi"}}{{.ip}}{{end}}{{end}}'`

echo "
apiVersion: storage.k8s.io/v1beta1
kind: StorageClass
metadata:
  name: glusterfs
provisioner: kubernetes.io/glusterfs
parameters:
  resturl: \"http://$ip:8080\"
  restuser: \"admin\"
  restuserkey: \"My Secret\"
" | kubectl create -f -

sleep 1

echo "
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
 name: gluster1
 annotations:
   volume.beta.kubernetes.io/storage-class: glusterfs
spec:
 accessModes:
  - ReadWriteMany
 resources:
   requests:
     storage: 5Gi
" | kubectl create -f -

