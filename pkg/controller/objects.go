package controller

import (
	"time"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	anthillapi "github.com/gluster/anthill/pkg/apis/anthill/v1alpha1"
)

func (c *Controller) newService(gc *anthillapi.GlusterCluster) *corev1.Service {
	port := corev1.ServicePort{
		Name:     "dummy",
		Protocol: corev1.ProtocolTCP,
		Port:     int32(1),
	}
	ownerRefs := []metav1.OwnerReference{
		*metav1.NewControllerRef(gc, schema.GroupVersionKind{
			Group:   anthillapi.SchemeGroupVersion.Group,
			Version: anthillapi.SchemeGroupVersion.Version,
			Kind:    "GlusterCluster",
		}),
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            gc.Name,
			Namespace:       gc.Namespace,
			OwnerReferences: ownerRefs,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector: map[string]string{
				"anthill": gc.Name + "-node",
			},
			Ports: []corev1.ServicePort{port},
		},
	}
}

func (c *Controller) newDeployment(node *anthillapi.Node) *appsv1.Deployment {
	newPod := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:            node.Name,
			OwnerReferences: node.OwnerReferences,
			Labels:          node.Labels,
		},
		Spec: node.Spec,
	}

	one := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            node.Name,
			Namespace:       node.Namespace,
			OwnerReferences: node.OwnerReferences,
			Labels:          node.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{
				MatchLabels: node.Labels,
			},
			Template: newPod,
		},
	}
}

func (c *Controller) waitForDeployment(d *appsv1.Deployment, timeout int32) error {
	return wait.Poll(2*time.Second, time.Duration(timeout)*time.Second, func() (bool, error) {
		deployment, err := c.kubeClientset.AppsV1().Deployments(d.Namespace).Get(d.Name, metav1.GetOptions{})
		if err != nil {
			glog.Error(err)
			return false, err
		}
		complete := deployment.Status.UpdatedReplicas == *(d.Spec.Replicas) &&
			deployment.Status.Replicas == *(d.Spec.Replicas) &&
			deployment.Status.AvailableReplicas == *(d.Spec.Replicas) &&
			deployment.Status.ObservedGeneration >= d.Generation
		return complete, nil
	})
}

func (c *Controller) newStorageClass(volume *anthillapi.NodeVolume) *storagev1.StorageClass {
	bindModeWait := storagev1.VolumeBindingWaitForFirstConsumer
	return &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:            volume.StorageClassName,
			OwnerReferences: volume.OwnerReferences,
		},
		Provisioner:       "kubernetes.io/no-provisioner",
		VolumeBindingMode: &bindModeWait,
	}
}

func (c *Controller) newPersistentVolume(volume *anthillapi.NodeVolume) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:            volume.Name,
			OwnerReferences: volume.OwnerReferences,
		},
		Spec: volume.PersistentVolumeSpec,
	}
}

func (c *Controller) newPersistentVolumeClaim(volume *anthillapi.NodeVolume) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:            volume.Name + "-claim",
			OwnerReferences: volume.OwnerReferences,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      volume.AccessModes,
			Selector:         &metav1.LabelSelector{MatchLabels: volume.Labels},
			VolumeName:       volume.Name,
			StorageClassName: &volume.StorageClassName,
			VolumeMode:       volume.VolumeMode,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: volume.Capacity[corev1.ResourceStorage],
				},
			},
		},
	}
}
