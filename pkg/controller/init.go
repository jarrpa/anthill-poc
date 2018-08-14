package controller

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"
	"github.com/imdario/mergo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"

	anthillapi "github.com/gluster/anthill/pkg/apis/anthill/v1alpha1"
)

// These values cannot be addressed in-line.
var oneGig, _ = resource.ParseQuantity("1Gi")
var cpuQuantity, _ = resource.ParseQuantity("100m")
var memQuantity, _ = resource.ParseQuantity("100Mi")
var blockMode = corev1.PersistentVolumeBlock
var trueBool = true
var deviceDir = "/dev/gluster/"

var GlusterClusterDefaults = map[string]interface{}{
	"Spec": map[string]interface{}{
		"PodTimeout": int32(300),
		"Cascade":    false,
		"Heketi": map[string]interface{}{
			"Native":    true,
			"DbStorage": "secret",
			"Node":      HeketiNodeDefaults,
			"Config":    HeketiConfigDefaults,
		},
		"GlusterFS": map[string]interface{}{
			"Native": true,
			"Provisioner": map[string]interface{}{
				"Name":               "gluster.org/glusterfile",
				"Image":              "gluster/glusterfileclone:latest",
				"CreateStorageClass": true,
			},
		},
		"GlusterBlock": map[string]interface{}{
			"Provisioner": map[string]interface{}{
				"Name":               "gluster.org/glusterblock",
				"Image":              "gluster/glusterblock-provisioner:latest",
				"CreateStorageClass": true,
			},
		},
	},
}

var HeketiConfigDefaults = map[string]interface{}{
	"Port":        "8080",
	"AuthEnabled": true,

	"BackupDbToKubeSecret": true,

	"JwtConfig": map[string]interface{}{
		"Admin": map[string]interface{}{
			"PrivateKey": "My Secret",
		},
		"User": map[string]interface{}{
			"PrivateKey": "My Secret",
		},
	},

	"GlusterFS": map[string]interface{}{
		"Executor": "kubernetes",
		"DBfile":   "/var/lib/heketi/heketi.db",

		"KubeConfig": map[string]interface{}{
			"Fstab":         "/var/lib/heketi/fstab",
			"SnapShopLimit": 14,
		},

		"CreateBlockHostingVolumes": true,
		"BlockHostingVolumeSize":    100,

		"IgnoreStaleOperations": true,
	},
}

var HeketiNodeDefaults = map[string]interface{}{
	"ServerType": "heketi",
	"Spec": map[string]interface{}{
		"Containers": []corev1.Container{
			corev1.Container{
				Name:  "heketi",
				Image: "heketi/heketi:dev",
				Ports: []corev1.ContainerPort{
					corev1.ContainerPort{ContainerPort: 8080},
				},
				Env: []corev1.EnvVar{
					corev1.EnvVar{Name: "HEKETI_CLI_SERVER", Value: "http://localhost:8080"},
					corev1.EnvVar{Name: "HEKETI_CLI_KEY", Value: "My Secret"},
					corev1.EnvVar{Name: "HEKETI_CLI_USER", Value: "admin"},
					/*
						corev1.EnvVar{
							Name: "HEKETI_CLI_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "",
									},
									Key:      "key",
									Optional: &trueBool,
								},
							},
						},
						corev1.EnvVar{
							Name: "HEKETI_ADMIN_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "",
									},
									Key:      "key",
									Optional: &trueBool,
								},
							},
						},
					*/
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    cpuQuantity,
						corev1.ResourceMemory: memQuantity,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					corev1.VolumeMount{Name: "config", MountPath: "/etc/heketi"},
					corev1.VolumeMount{Name: "backupdb", MountPath: "/backupdb"},
				},
				VolumeDevices: []corev1.VolumeDevice{},
				LivenessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(3),
					TimeoutSeconds:      int32(3),
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/hello",
							Port: intstr.FromInt(8080),
						},
					},
				},
				ReadinessProbe: &corev1.Probe{
					InitialDelaySeconds: int32(30),
					TimeoutSeconds:      int32(3),
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/hello",
							Port: intstr.FromInt(8080),
						},
					},
				},
				//ImagePullPolicy: corev1.PullIfNotPresent,
				ImagePullPolicy: corev1.PullAlways,
			},
		},
	},
}

var GlusterNodeDefaults = map[string]interface{}{
	"ServerType": "gluster",
	"StateVolume": map[string]interface{}{
		"Provision": true,
		"Capacity": map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceStorage: oneGig,
		},
		"AccessModes": []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		"HostPath": &corev1.HostPathVolumeSource{
			Path: "/var/lib/glusterfs-containers",
		},
	},
	"Spec": map[string]interface{}{
		"InitContainers": []corev1.Container{},
		"Containers":     []corev1.Container{},
		"Volumes": []corev1.Volume{
			corev1.Volume{
				Name: "run",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			corev1.Volume{
				Name: "lvm",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/run/lvm",
					},
				},
			},
			corev1.Volume{
				Name: "dev",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/dev",
					},
				},
			},
			corev1.Volume{
				Name: "blkdevbridge",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			},
		},
	},
	"Zone": 1,
}

var InitContainerDefaults = corev1.Container{
	Name:    "blkdevmapper",
	Image:   "busybox:latest",
	Command: []string{"/bin/sh", "-c"},
	Args:    []string{"cp -a $(DEVICES) /mnt"},
	VolumeMounts: []corev1.VolumeMount{
		corev1.VolumeMount{Name: "blkdevbridge", MountPath: "/mnt"},
	},
}

var GlusterContainerDefaults = corev1.Container{
	Name:  "glusterfs",
	Image: "jarrpa/gluster-fedora-minimal:dev",
	Ports: []corev1.ContainerPort{
		corev1.ContainerPort{ContainerPort: 3260},
		corev1.ContainerPort{ContainerPort: 24006},
		corev1.ContainerPort{ContainerPort: 24007},
	},
	Env: []corev1.EnvVar{
		corev1.EnvVar{
			Name: "GLUSTERFS_NODE_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.labels['name']",
				},
			},
		},
	},
	Resources: corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cpuQuantity,
			corev1.ResourceMemory: memQuantity,
		},
	},
	VolumeMounts: []corev1.VolumeMount{
		corev1.VolumeMount{Name: "gluster-state", MountPath: "/glusterfs"},
		corev1.VolumeMount{Name: "kernel-modules", MountPath: "/usr/lib/modules", ReadOnly: true},
		corev1.VolumeMount{Name: "run", MountPath: "/run"},
		//corev1.VolumeMount{Name: "lvm", MountPath: "/run/lvm"},
		//corev1.VolumeMount{Name: "dev", MountPath: "/dev"},
		corev1.VolumeMount{Name: "blkdevbridge", MountPath: deviceDir},
	},
	LivenessProbe: &corev1.Probe{
		InitialDelaySeconds: int32(30),
		TimeoutSeconds:      int32(3),
		PeriodSeconds:       int32(10),
		SuccessThreshold:    int32(1),
		FailureThreshold:    int32(50),
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(24007)},
		},
	},
	ReadinessProbe: &corev1.Probe{
		InitialDelaySeconds: int32(30),
		TimeoutSeconds:      int32(3),
		PeriodSeconds:       int32(10),
		SuccessThreshold:    int32(1),
		FailureThreshold:    int32(50),
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(24007)},
		},
	},
	SecurityContext: &corev1.SecurityContext{
		Privileged:   &trueBool,
		Capabilities: &corev1.Capabilities{Add: []corev1.Capability{corev1.Capability("SYS_MODULE")}},
	},
	//ImagePullPolicy: corev1.PullIfNotPresent,
	ImagePullPolicy: corev1.PullAlways,
}

var GlusterNodeVolumeDefaults = map[string]interface{}{
	"Provision": true,
	"Capacity": map[corev1.ResourceName]resource.Quantity{
		corev1.ResourceStorage: oneGig,
	},
	"AccessModes": []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
	"VolumeMode":  &blockMode,
}

func (c *Controller) initGlusterCluster(gc *anthillapi.GlusterCluster) (*anthillapi.GlusterCluster, error) {
	if gc == nil {
		return nil, nil
	}

	glog.Infof("Initializing GlusterCluster %s", gc.Name)

	gcCopy := gc.DeepCopy()

	if err := mergo.Map(gcCopy, GlusterClusterDefaults); err != nil {
		err = fmt.Errorf("mergo error: %v", err)
		glog.Error(err)
		return nil, err
	}

	gcCopy.Labels = map[string]string{
		"anthill": gc.Name,
		gc.Name:   "cluster",
	}

	if err := c.initHeketiNode(gcCopy); err != nil {
		return nil, err
	}

	for nIdx, node := range gcCopy.Spec.Nodes {
		if err := c.initGlusterNode(gcCopy, node, nIdx); err != nil {
			return nil, err
		}

		for dIdx, device := range node.Devices {
			if err := c.initGlusterNodeVolume(node, device, dIdx); err != nil {
				return nil, err
			}
		}
	}

	return gcCopy, nil
}

func (c *Controller) initHeketiNode(gc *anthillapi.GlusterCluster) error {
	var node = gc.Spec.Heketi.Node

	glog.Infof("Initializing GlusterCluster %s heketi node", gc.Name)

	if node.Timeout == int32(0) {
		node.Timeout = gc.Spec.PodTimeout
	}
	if node.Name == "" {
		node.Name = gc.Name + "-heketi"
	}

	if node.Namespace == "" {
		node.Namespace = gc.Namespace
	}

	node.Spec.Hostname = node.Name
	node.Spec.Subdomain = gc.Name

	node.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(gc, schema.GroupVersionKind{
			Group:   anthillapi.SchemeGroupVersion.Group,
			Version: anthillapi.SchemeGroupVersion.Version,
			Kind:    "GlusterCluster",
		}),
	}

	node.Labels = map[string]string{
		"name":    node.Name,
		"anthill": gc.Name + "-node",
		gc.Name:   "heketi-node",
	}

	node.Spec.ServiceAccountName = node.Name + "-service-account"

	configVol := corev1.Volume{
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: node.Name + "-config"},
			},
		},
	}

	node.Spec.Volumes = append(node.Spec.Volumes, configVol)

	dbSecretName := node.Name + "-db-secret"
	dbBackupVol := corev1.Volume{
		Name: "backupdb",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: dbSecretName,
			},
		},
	}

	node.Spec.Volumes = append(node.Spec.Volumes, dbBackupVol)

	dbSecretVar := corev1.EnvVar{Name: "HEKETI_KUBE_DB_SECRET_NAME", Value: dbSecretName}
	node.Spec.Containers[0].Env = append(node.Spec.Containers[0].Env, dbSecretVar)

	return nil
}

func (c *Controller) initGlusterNode(gc *anthillapi.GlusterCluster, node *anthillapi.Node, nIdx int) error {
	var err error

	glog.Infof("Initializing GlusterCluster %s node %d", gc.Name, nIdx)

	if err = mergo.Map(node, GlusterNodeDefaults); err != nil {
		err := fmt.Errorf("mergo error: %v", err)
		glog.Error(err)
		return err
	}

	if node.Timeout == int32(0) {
		node.Timeout = gc.Spec.PodTimeout
	}
	if node.Name == "" {
		node.Name = gc.Name + "-" + strconv.Itoa(nIdx)
	}

	node.Spec.Hostname = node.Name
	node.Spec.Subdomain = gc.Name

	if node.Namespace == "" {
		node.Namespace = gc.Namespace
	}

	node.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(gc, schema.GroupVersionKind{
			Group:   anthillapi.SchemeGroupVersion.Group,
			Version: anthillapi.SchemeGroupVersion.Version,
			Kind:    "GlusterCluster",
		}),
	}

	node.Labels = map[string]string{
		"name":           node.Name,
		"anthill":        gc.Name + "-node",
		gc.Name:          "gluster-node",
		"glusterfs-node": node.Name + "." + gc.Name + "." + gc.Namespace + ".svc",
	}

	var stateVol = node.StateVolume

	if stateVol.Name == "" {
		stateVol.Name = node.Name + "-state"
	}

	stateVol.OwnerReferences = node.OwnerReferences
	stateVol.Namespace = node.Namespace

	stateVol.Labels = map[string]string{
		"anthill": gc.Name + "-" + node.Name + "-volume",
		gc.Name:   node.Name + "-volume",
	}

	stateVolSource := corev1.Volume{
		Name: "gluster-state",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: stateVol.Name + "-claim"},
		},
	}

	node.Spec.Volumes = append(node.Spec.Volumes, stateVolSource)

	kernelMods := corev1.Volume{
		Name: "kernel-modules",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: node.Name + "-kernel-modules-claim"},
		},
	}

	node.Spec.Volumes = append(node.Spec.Volumes, kernelMods)

	node.Spec.Containers = append(node.Spec.Containers, GlusterContainerDefaults)

	node.Spec.InitContainers = append(node.Spec.InitContainers, InitContainerDefaults)

	return nil
}

func (c *Controller) initGlusterNodeVolume(node *anthillapi.Node, volume *anthillapi.NodeVolume, dIdx int) error {
	var err error

	if err = mergo.Map(volume, GlusterNodeVolumeDefaults); err != nil {
		err := fmt.Errorf("mergo error: %v", err)
		glog.Error(err)
		return err
	}

	if volume.Name == "" {
		volume.Name = node.Name + "-" + strconv.Itoa(dIdx)
	}
	if volume.StorageClassName == "" {
		volume.StorageClassName = node.Name + "-volumes"
	}

	volume.OwnerReferences = node.OwnerReferences
	volume.Namespace = node.Namespace

	owner := volume.OwnerReferences[0].Name
	volume.Labels = map[string]string{
		"anthill": owner + "-" + node.Name + "-device",
		owner:     node.Name + "-device",
	}

	volSource := corev1.Volume{
		Name: volume.Name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: volume.Name + "-claim"},
		},
	}
	node.Spec.Volumes = append(node.Spec.Volumes, volSource)

	volDevice := corev1.VolumeDevice{
		Name:       volume.Name,
		DevicePath: deviceDir + volume.Name,
	}
	node.Spec.Containers[0].VolumeDevices = append(node.Spec.Containers[0].VolumeDevices, volDevice)
	node.Spec.InitContainers[0].VolumeDevices = append(node.Spec.InitContainers[0].VolumeDevices, volDevice)

	if len(node.Spec.InitContainers[0].Env) == 0 {
		node.Spec.InitContainers[0].Env = []corev1.EnvVar{
			corev1.EnvVar{Name: "DEVICES"},
		}
	}
	node.Spec.InitContainers[0].Env[0].Value += " " + volDevice.DevicePath

	return nil
}
