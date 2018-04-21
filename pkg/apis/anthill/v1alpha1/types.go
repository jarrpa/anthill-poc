/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"github.com/heketi/heketi/middleware"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlusterCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   *GlusterClusterSpec   `json:"spec"`
	Status *GlusterClusterStatus `json:"status,omitempty"`
}

type GlusterClusterSpec struct {
	PodTimeout int32 `json:"podTimeout,omitempty"`
	Cascade    bool  `json:"cascade,omitempty"`
	Wipe       bool  `json:"wipe,omitempty"`

	Nodes []*Node `json:"nodes,omitempty"`

	Heketi       *HeketiSpec       `json:"heketi,omitempty"`
	GlusterFS    *GlusterfsSpec    `json:"glusterfs,omitempty"`
	GlusterBlock *GlusterBlockSpec `json:"glusterblock,omitempty"`
}

type Node struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Timeout int32 `json:"timeout,omitempty"`

	ServerType string `json:"serverType,omitempty"`
	IPAddr     string `json:"ipAddress,omitempty"`
	Zone       int    `json:"zone,omitempty"`

	Devices     []*NodeVolume  `json:"devices"`
	StateVolume *NodeVolume    `json:"stateVolume",omitempty`
	Spec        corev1.PodSpec `json:"spec"`
}

type NodeVolume struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Provision bool `json:"provision,omitempty"`

	corev1.PersistentVolumeSpec `json:",inline"`
}

type HeketiSpec struct {
	Native bool   `json:"native,omitempty"`
	Image  string `json:"image,omitempty"`

	AdminSecret string `json:"adminSecret,omitempty"`

	Config *HeketiConfigSpec `json:"config,omitempty"`
	Node   *Node             `json:"node,omitempty"`
}

type HeketiConfigSpec struct {
	Port string `json:"port"`

	AuthEnabled bool                     `json:"use_auth"`
	JwtConfig   middleware.JwtAuthConfig `json:"jwt"`

	BackupDbToKubeSecret bool `json:"backup_db_to_kube_secret"`

	GlusterFS GlusterFSConfig `json:"glusterfs"`
}

type GlusterFSConfig struct {
	DBfile     string     `json:"db"`
	Executor   string     `json:"executor"`
	Allocator  string     `json:"allocator"`
	SshConfig  SshConfig  `json:"sshexec"`
	KubeConfig KubeConfig `json:"kubeexec"`
	Loglevel   string     `json:"loglevel"`

	// advanced settings
	BrickMaxSize int `json:"brick_max_size_gb"`
	BrickMinSize int `json:"brick_min_size_gb"`
	BrickMaxNum  int `json:"max_bricks_per_volume"`

	//block settings
	CreateBlockHostingVolumes bool `json:"auto_create_block_hosting_volume"`
	BlockHostingVolumeSize    int  `json:"block_hosting_volume_size"`
}
type KubeConfig struct {
	CmdConfig

	Namespace        string `json:"namespace"`
	GlusterDaemonSet bool   `json:"gluster_daemonset"`

	// Use POD name instead of using label
	// to access POD
	UsePodNames bool `json:"use_pod_names"`
}
type SshConfig struct {
	CmdConfig

	PrivateKeyFile string `json:"keyfile"`
	User           string `json:"user"`
	Port           string `json:"port"`
}
type CmdConfig struct {
	Fstab                string `json:"fstab"`
	Sudo                 bool   `json:"sudo"`
	SnapShotLimit        int    `json:"snapshot_limit"`
	RebalanceOnExpansion bool   `json:"rebalance_on_expansion"`
}

type HeketiSSHSpec struct {
	Port    int32  `json:"port,omitempty"`
	User    string `json:"user,omitempty"`
	KeyFile string `json:"keyFile,omitempty"`
}

type GlusterfsSpec struct {
	Native bool   `json:"native,omitempty"`
	Image  string `json:"image,omitempty"`

	Provisioner *VolumeProvisioner `json:"provisioner,omitempty"`
}

type GlusterBlockSpec struct {
	Provisioner *VolumeProvisioner `json:"provisioner,omitempty"`
}

type VolumeProvisioner struct {
	Name  string `json:"name,omitempty"`
	Image string `json:"image,omitempty"`

	CreateStorageClass bool `json:"createStorageClass,omitempty"`
}

type GlusterClusterStatus struct {
	Deployed bool `json:"deployed"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GlusterClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GlusterCluster `json:"items"`
}
