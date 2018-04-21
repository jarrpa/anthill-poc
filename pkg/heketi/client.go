package heketi

import (
	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	anthillapi "github.com/gluster/anthill/pkg/apis/anthill/v1alpha1"
	heketiclient "github.com/heketi/heketi/client/api/go-client"
	heketiapi "github.com/heketi/heketi/pkg/glusterfs/api"
)

type HeketiClient struct {
	Url        string
	AdminKey   string
	ClusterId  string
	KubeClient kubernetes.Interface
	Deployment *appsv1.Deployment
	Client     *heketiclient.Client
}

func NewClient(gc *anthillapi.GlusterCluster, kcs kubernetes.Interface) (*HeketiClient, error) {
	deployment, err := kcs.AppsV1().Deployments(gc.Namespace).Get(gc.Spec.Heketi.Node.Name, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	newClient := &HeketiClient{
		Url:        "http://" + gc.Spec.Heketi.Node.Name + "." + gc.Name + ":" + gc.Spec.Heketi.Config.Port,
		AdminKey:   gc.Spec.Heketi.Config.JwtConfig.Admin.PrivateKey,
		ClusterId:  deployment.Labels["heketiClusterId"],
		KubeClient: kcs,
		Deployment: deployment,
	}

	newClient.Client = heketiclient.NewClient(newClient.Url, "admin", newClient.AdminKey)

	return newClient, nil
}

func (h *HeketiClient) CreateCluster() error {
	if h.ClusterId != "" {
		list, err := h.Client.ClusterList()
		if err != nil {
			glog.Error(err)
			return err
		}
		for _, cluster := range list.Clusters {
			if cluster == h.ClusterId {
				return nil
			}
		}
	}

	req := &heketiapi.ClusterCreateRequest{
		ClusterFlags: heketiapi.ClusterFlags{
			Block: true,
			File:  true,
		},
	}

	clusterInfo, err := h.Client.ClusterCreate(req)
	if err != nil {
		return err
	}

	h.ClusterId = clusterInfo.Id

	h.Deployment.Labels["heketiClusterId"] = h.ClusterId

	h.Deployment, err = h.KubeClient.AppsV1().Deployments(h.Deployment.Namespace).Update(h.Deployment)
	if err != nil {
		return err
	}

	glog.Infof("Heketi: Created cluster %s", clusterInfo.Id)

	return nil
}

func (h *HeketiClient) GetCluster(clusterId string) (*heketiapi.ClusterInfoResponse, error) {
	glog.Infof("Heketi: Getting cluster info for %s", clusterId)
	clusterInfo, err := h.Client.ClusterInfo(clusterId)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	return clusterInfo, nil
}

func (h *HeketiClient) AddNode(node *anthillapi.Node) error {
	deployment, err := h.KubeClient.AppsV1().Deployments(node.Namespace).Get(node.Name, metav1.GetOptions{})
	if err != nil {
		glog.Error(err)
		return err
	}

	if deployment.Labels["heketiNodeId"] != "" && deployment.Labels["heketiClusterId"] == h.ClusterId {
		clusterInfo, err := h.GetCluster(h.ClusterId)
		if err != nil {
			return err
		}
		for _, n := range clusterInfo.Nodes {
			if n == deployment.Labels["heketiNodeId"] {
				nodeInfo, err := h.GetNode(n)
				if err != nil {
					return err
				}
				for _, device := range deployment.Spec.Template.Spec.Containers[0].VolumeDevices {
					if err := h.AddDevice(device.DevicePath, nodeInfo); err != nil {
						return err
					}
				}
				return nil
			}
		}
	}

	req := &heketiapi.NodeAddRequest{}
	req.ClusterId = h.ClusterId
	req.Hostnames.Manage = []string{node.Labels["glusterfs-node"]}
	req.Hostnames.Storage = []string{node.Labels["glusterfs-node"]}
	req.Zone = node.Zone

	nodeInfo, err := h.Client.NodeAdd(req)
	if err != nil {
		glog.Error(err)
		return err
	}

	deployment.Labels["heketiClusterId"] = nodeInfo.ClusterId
	deployment.Labels["heketiNodeId"] = nodeInfo.Id

	deployment, err = h.KubeClient.AppsV1().Deployments(deployment.Namespace).Update(deployment)

	if err != nil {
		glog.Error(err)
		return err
	}

	glog.Infof("Heketi: Node %s added as %s", node.Name, nodeInfo.Id)

	for _, device := range deployment.Spec.Template.Spec.Containers[0].VolumeDevices {
		if err := h.AddDevice(device.DevicePath, nodeInfo); err != nil {
			return err
		}
	}

	return nil
}

func (h *HeketiClient) GetNode(nodeId string) (*heketiapi.NodeInfoResponse, error) {
	glog.Infof("Heketi: Getting node info for %s", nodeId)
	nodeInfo, err := h.Client.NodeInfo(nodeId)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	return nodeInfo, nil
}

func (h *HeketiClient) AddDevice(device string, nodeInfo *heketiapi.NodeInfoResponse) error {
	for _, devInfo := range nodeInfo.DevicesInfo {
		if devInfo.Name == device {
			return nil
		}
	}

	glog.Infof("Heketi: Adding node %s device %s", nodeInfo.Id, device)
	req := &heketiapi.DeviceAddRequest{}
	req.NodeId = nodeInfo.Id
	req.Name = device

	if err := h.Client.DeviceAdd(req); err != nil {
		glog.Error(err)
		return err
	}

	return nil
}
