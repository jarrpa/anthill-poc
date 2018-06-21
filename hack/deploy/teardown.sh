#!/bin/bash

kubectl delete storageclass.storage.k8s.io/glusterfs
kubectl delete persistentvolumeclaim/gluster1
kubectl delete -n anthill -f Anthill.yaml

ssh node0 -- sudo rm -rf /var/lib/glusterfs-containers/*
ssh node1 -- sudo rm -rf /var/lib/glusterfs-containers/*
ssh node2 -- sudo rm -rf /var/lib/glusterfs-containers/*
