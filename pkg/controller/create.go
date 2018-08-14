package controller

import (
	"encoding/json"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	anthillapi "github.com/gluster/anthill/pkg/apis/anthill/v1alpha1"
)

func (c *Controller) createService(gc *anthillapi.GlusterCluster) error {
	var err error

	glog.Infof("Creating Service %s", gc.Name)

	_, err = c.kubeClientset.CoreV1().Services(gc.Namespace).Create(c.newService(gc))

	if err != nil && !errors.IsAlreadyExists(err) {
		glog.Error(err)
		return err
	}

	return nil
}

func (c *Controller) createHeketiConfig(heketi *anthillapi.HeketiSpec) error {
	var err error
	var node = heketi.Node

	b, err := json.Marshal(heketi.Config)

	if err != nil {
		glog.Error(err)
		return err
	}

	newCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            node.Name + "-config",
			Namespace:       node.Namespace,
			OwnerReferences: node.OwnerReferences,
			Labels: map[string]string{
				"anthill": node.Name + "-config",
				"heketi":  "config",
			},
		},
		Data: map[string]string{
			"heketi.json": string(b),
		},
	}

	_, err = c.kubeClientset.CoreV1().ConfigMaps(node.Namespace).Create(newCM)

	if err != nil {
		if !errors.IsAlreadyExists(err) {
			glog.Error(err)
			return err
		}
	}

	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name + "-db-secret",
			Labels: map[string]string{
				"deploy-heketi": "support",
			},
		},
		Data: map[string][]byte{
			"heketi.db": []byte{},
		},
	}

	_, err = c.kubeClientset.CoreV1().Secrets(node.Namespace).Create(newSecret)

	if err != nil {
		if !errors.IsAlreadyExists(err) {
			glog.Error(err)
			return err
		}
	}

	return nil
}

func (c *Controller) createHeketiCredentials(node *anthillapi.Node) error {
	var err error

	newSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            node.Name + "-service-account",
			Namespace:       node.Namespace,
			OwnerReferences: node.OwnerReferences,
			Labels: map[string]string{
				"anthill": node.Name + "-service-account",
				"heketi":  "service-account",
			},
		},
	}

	_, err = c.kubeClientset.CoreV1().ServiceAccounts(node.Namespace).Create(newSA)

	if err != nil {
		if !errors.IsAlreadyExists(err) {
			glog.Error(err)
			return err
		}
	}

	newRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            node.Name + "-role-binding",
			Namespace:       node.Namespace,
			OwnerReferences: node.OwnerReferences,
			Labels: map[string]string{
				"anthill": node.Name + "-role-binding",
				"heketi":  "role-binding",
			},
		},
		Subjects: []rbacv1.Subject{
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      node.Name + "-service-account",
				Namespace: node.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "anthill",
		},
	}

	_, err = c.kubeClientset.RbacV1().RoleBindings(node.Namespace).Create(newRB)

	if err != nil {
		if !errors.IsAlreadyExists(err) {
			glog.Error(err)
			return err
		}
	}

	return nil
}

func (c *Controller) createDeviceVolume(n *anthillapi.Node, dIdx int) error {
	var err error
	var d = n.Devices[dIdx]

	if err = c.createVolumeResources(d); err != nil {
		return err
	}

	return nil
}

func (c *Controller) createGlusterMountVolumes(node *anthillapi.Node) error {
	var err error
	var stateVol = node.StateVolume

	glog.Infof("Creating node %s state volume", node.Name)
	if err = c.createVolumeResources(stateVol); err != nil {
		return err
	}

	kernelModVol := anthillapi.NodeVolume{

		ObjectMeta: metav1.ObjectMeta{
			Name:            node.Name + "-kernel-modules",
			Namespace:       node.Namespace,
			OwnerReferences: node.OwnerReferences,
			Labels:          stateVol.Labels,
		},
		Provision: true,
		PersistentVolumeSpec: corev1.PersistentVolumeSpec{
			Capacity: map[corev1.ResourceName]resource.Quantity{
				corev1.ResourceStorage: oneGig,
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/usr/lib/modules",
				},
			},
			StorageClassName: "",
		},
	}

	glog.Infof("Creating node %s kernel modules volume", node.Name)
	if err = c.createVolumeResources(&kernelModVol); err != nil {
		return err
	}

	return nil
}

func (c *Controller) createVolumeResources(d *anthillapi.NodeVolume) error {
	if d.Provision {
		if d.StorageClassName != "" {
			if err := c.createStorageClass(d); err != nil {
				return err
			}
		}
		if err := c.createPV(d); err != nil {
			return err
		}
	}

	if err := c.createPVC(d); err != nil {
		return err
	}

	return nil
}

func (c *Controller) createStorageClass(d *anthillapi.NodeVolume) error {
	_, err := c.kubeClientset.StorageV1().StorageClasses().Create(c.newStorageClass(d))

	if err != nil && !errors.IsAlreadyExists(err) {
		glog.Error(err)
		return err
	}

	return nil
}

func (c *Controller) createPV(vol *anthillapi.NodeVolume) error {
	glog.Infof("Creating PV %s", vol.Name)

	_, err := c.kubeClientset.CoreV1().PersistentVolumes().Create(c.newPersistentVolume(vol))

	if err != nil && !errors.IsAlreadyExists(err) {
		glog.Error(err)
		return err
	}

	return nil
}

func (c *Controller) createPVC(vol *anthillapi.NodeVolume) error {
	glog.Infof("Creating PVC %s", vol.Name)

	_, err := c.kubeClientset.CoreV1().PersistentVolumeClaims(vol.Namespace).Create(c.newPersistentVolumeClaim(vol))

	if err != nil && !errors.IsAlreadyExists(err) {
		glog.Error(err)
		return err
	}

	return nil
}

func (c *Controller) createDeployment(node *anthillapi.Node) error {
	var err error

	glog.Infof("Creating Deployment %s", node.Name)

	deployment, err := c.kubeClientset.AppsV1().Deployments(node.Namespace).Create(c.newDeployment(node))

	if err != nil {
		if !errors.IsAlreadyExists(err) {
			glog.Error(err)
			return err
		}
	} else {
		if err := c.waitForDeployment(deployment, node.Timeout); err != nil {
			return err
		}
	}

	return nil
}
