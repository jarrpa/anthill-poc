package controller

import (
	"reflect"
	"sync"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	anthillapi "github.com/gluster/anthill/pkg/apis/anthill/v1alpha1"
	"github.com/gluster/anthill/pkg/heketi"
)

func (c *Controller) updateGlusterCluster(old *anthillapi.GlusterCluster, new *anthillapi.GlusterCluster) error {
	var newNodes []*anthillapi.Node
	var outChan = make(chan error)
	var wg = sync.WaitGroup{}

	if err := c.createService(new); err != nil {
		return err
	}

	if old == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.createHeketiConfig(new.Spec.Heketi); err != nil {
				outChan <- err
			}
			if err := c.deployNode(new, new.Spec.Heketi.Node); err != nil {
				outChan <- err
			}
			outChan <- nil
		}()
		newNodes = new.Spec.Nodes
	} else if !reflect.DeepEqual(old.Spec.Nodes, new.Spec.Nodes) {
		oldNodesLen := len(old.Spec.Nodes)
		newNodesLen := len(new.Spec.Nodes)

		i := 0
		for ; i < oldNodesLen && i < newNodesLen; i++ {
			if !reflect.DeepEqual(old.Spec.Nodes[i], new.Spec.Nodes[i]) {
				if err := c.updateGlusterNode(new, old.Spec.Nodes[i], new.Spec.Nodes[i]); err != nil {
					return err
				}
			}
		}

		if newNodesLen != oldNodesLen {
			if newNodesLen > oldNodesLen {
				for ; i < newNodesLen; i++ {
					newNodes = append(newNodes, new.Spec.Nodes[i])
				}
			} else if oldNodesLen > newNodesLen {
				//TODO: delete nodes
			}
		}
	}

	if new.Spec.GlusterFS.Native {
		for _, node := range newNodes {
			wg.Add(1)
			go func(n *anthillapi.Node) {
				defer wg.Done()
				err := c.deployNode(new, n)
				outChan <- err
			}(node)
		}
	}

	go func() {
		wg.Wait()
		close(outChan)
	}()

	for err := range outChan {
		if err != nil {
			return err
		}
	}

	heketiClient, err := heketi.NewClient(new, c.kubeClientset)
	if err != nil {
		return err
	}

	if err := heketiClient.CreateCluster(); err != nil {
		return err
	}

	for _, node := range newNodes {
		if err := heketiClient.AddNode(node); err != nil {
			return err
		}
	}

	/*
		if gc.Deploy.FileStorageClass {
		  err = deployFileSC()
		  iferr
		}
		if gc.Deploy.BlockStorageClass {
		  err = deployBlockSC()
		  iferr
		}
	*/
	return nil
}

type UpdateVolume struct {
	Old   *anthillapi.NodeVolume
	New   *anthillapi.NodeVolume
	Index int
	State bool
}

func (c *Controller) updateGlusterNode(gc *anthillapi.GlusterCluster, old *anthillapi.Node, new *anthillapi.Node) error {
	var emptyVol UpdateVolume
	var updateVolumes []UpdateVolume

	if upVol := c.diffGlusterNodeVolume(old.StateVolume, new.StateVolume, 0); upVol != emptyVol {
		updateVolumes = append(updateVolumes, upVol)
		updateVolumes[0].State = true
	}

	oldDevicesLen := len(old.Devices)
	newDevicesLen := len(new.Devices)

	i := 0
	for ; i < oldDevicesLen && i < newDevicesLen; i++ {
		if upVol := c.diffGlusterNodeVolume(old.Devices[i], new.Devices[i], i); upVol != emptyVol {
			updateVolumes = append(updateVolumes, upVol)
		}
	}

	if newDevicesLen != oldDevicesLen {
		if newDevicesLen > oldDevicesLen {
			for ; i < newDevicesLen; i++ {
				upVol := UpdateVolume{
					Old:   nil,
					New:   new.Devices[i],
					Index: i,
				}
				updateVolumes = append(updateVolumes, upVol)
			}
		} else if oldDevicesLen > newDevicesLen {
			for ; i < oldDevicesLen; i++ {
				upVol := UpdateVolume{
					Old:   old.Devices[i],
					New:   nil,
					Index: i,
				}
				updateVolumes = append(updateVolumes, upVol)
			}
		}
	}

	if len(updateVolumes) != 0 {

		deployment, err := c.kubeClientset.AppsV1().Deployments(new.Namespace).Get(new.Name, metav1.GetOptions{})
		if err != nil {
			glog.Error(err)
			return err
		}

		// TODO: Bring down node

		for _, upVol := range updateVolumes {
			if err = c.updateGlusterNodeVolume(upVol); err != nil {
				return err
			}
		}

		deployment.Spec.Template.Spec.Volumes = new.Spec.Volumes
		deployment.Spec.Template.Spec.Containers[0].VolumeDevices = new.Spec.Containers[0].VolumeDevices

		deployment, err = c.kubeClientset.AppsV1().Deployments(new.Namespace).Update(deployment)
		if err != nil {
			glog.Error(err)
			return err
		}

		if err := c.waitForDeployment(deployment, new.Timeout); err != nil {
			return err
		}

		if gc.Spec.Wipe {
			var newDevices []string
			for _, upVol := range updateVolumes {
				if upVol.New != nil {
					newDevices = append(newDevices, "/dev/"+upVol.New.Name)
				}
			}
			if err := c.wipeDevices(new, newDevices); err != nil {
				return err
			}
		}

		heketiClient, err := heketi.NewClient(gc, c.kubeClientset)
		if err != nil {
			return err
		}

		nodeInfo, err := heketiClient.GetNode(deployment.Labels["heketiNodeId"])
		if err != nil {
			return err
		}

		for _, device := range deployment.Spec.Template.Spec.Containers[0].VolumeDevices {
			if err := heketiClient.AddDevice(device.DevicePath, nodeInfo); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Controller) diffGlusterNodeVolume(old *anthillapi.NodeVolume, new *anthillapi.NodeVolume, idx int) UpdateVolume {
	var upVol UpdateVolume
	var update = false

	if old != nil && new != nil {
		update = old.Name != new.Name || update
		update = old.Capacity[corev1.ResourceStorage] != new.Capacity[corev1.ResourceStorage] || update
		update = old.PersistentVolumeReclaimPolicy != new.PersistentVolumeReclaimPolicy || update
		update = old.StorageClassName != new.StorageClassName || update

		update = !reflect.DeepEqual(old.AccessModes, new.AccessModes) || update
		update = !reflect.DeepEqual(old.MountOptions, new.MountOptions) || update
		update = !reflect.DeepEqual(old.NodeAffinity, new.NodeAffinity) || update
		update = !reflect.DeepEqual(old.PersistentVolumeSource, new.PersistentVolumeSource) || update
	} else {
		update = true
	}

	if update {
		upVol = UpdateVolume{
			Old:   old,
			New:   new,
			Index: idx,
			State: false,
		}
	}

	return upVol
}

func (c *Controller) updateGlusterNodeVolume(upVol UpdateVolume) error {
	if upVol.Old != nil {
		// TODO: Remove volume
	}
	if upVol.New != nil {
		if err := c.createVolumeResources(upVol.New); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) updateGlusterClusterStatus(gc *anthillapi.GlusterCluster, status *anthillapi.GlusterClusterStatus) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		gc.Status = status

		gc, err := c.anthillClient.GlusterV1alpha1().GlusterClusters(gc.Namespace).Update(gc)
		if err != nil {
			glog.Errorf("error updating GlusterCluster %s/%s: %v", gc.Namespace, gc.Name, err)
			if updated, getErr := c.glusterLister.GlusterClusters(gc.Namespace).Get(gc.Name); getErr == nil {
				gc = updated.DeepCopy()
			} else {
				glog.Errorf("error getting updated GlusterCluster %s/%s: %v", gc.Namespace, gc.Name, getErr)
			}
		}

		return err
	})

	if err != nil {
		glog.Error(err)
	}

	return err
}
