package controller

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/scheme"
	coreinternalversion "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd"

	anthillapi "github.com/gluster/anthill/pkg/apis/anthill/v1alpha1"
)

/*
func (c *Controller) deployGlusterCluster(gc *anthillapi.GlusterCluster) error {
	var err error

	var wg sync.WaitGroup
	createOut := make(chan error)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := c.createHeketiConfig(gc.Spec.Heketi); err != nil {
			createOut <- err
		}
		if err := c.deployNode(gc, gc.Spec.Heketi.Node); err != nil {
			createOut <- err
		}
		createOut <- nil
	}()

	if gc.Spec.GlusterFS.Native {
		for _, node := range gc.Spec.Nodes {
			wg.Add(1)
			go func(node *anthillapi.Node) {
				defer wg.Done()
				err := c.deployNode(gc, node)
				createOut <- err
			}(node)
		}
	}

	go func() {
		wg.Wait()
		close(createOut)
	}()

	for err = range createOut {
		if err != nil {
			return err
		}
	}

	glog.Infof("Creating heketi topology")
	heketiClient, err := heketi.NewClient(gc, c.kubeClientset)
	if err != nil {
		return err
	}

	if err = heketiClient.CreateCluster(); err != nil {
		return err
	}

	for _, node := range gc.Spec.Nodes {
		if err = heketiClient.AddNode(node); err != nil {
			return err
		}
	}

	return nil
}
*/
func (c *Controller) deployNode(gc *anthillapi.GlusterCluster, node *anthillapi.Node) error {
	glog.Infof("Processing node %s", node.Name)

	switch node.ServerType {
	case "gluster":
		if err := c.createGlusterMountVolumes(node); err != nil {
			return err
		}
		for _, device := range node.Devices {
			glog.Infof("Processing node %s device %s", node.Name, device.Name)
			if err := c.createVolumeResources(device); err != nil {
				return err
			}
		}
	case "heketi":
		if err := c.createHeketiCredentials(node); err != nil {
			return err
		}
	}

	if err := c.createDeployment(node); err != nil {
		return err
	}

	if node.ServerType == "gluster" && gc.Spec.Wipe {
		var devices []string
		for _, device := range node.Spec.Containers[0].VolumeDevices {
			devices = append(devices, device.DevicePath)
		}
		if err := c.wipeDevices(node, devices); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) wipeDevices(node *anthillapi.Node, devices []string) error {
	pods, err := c.kubeClientset.CoreV1().Pods(node.Namespace).List(metav1.ListOptions{LabelSelector: "name=" + node.Name})
	if err != nil {
		glog.Error(err)
		return err
	}
	if len(pods.Items) == 0 {
		return fmt.Errorf("Node %s pod not found", node.Name)
	}
	pod := pods.Items[0]

	configShallowCopy := *c.kubeConfig

	kubectlCoreClient, err := coreinternalversion.NewForConfig(&configShallowCopy)
	if err != nil {
		glog.Error(err)
		return err
	}

	if err := setConfigDefaults(&configShallowCopy); err != nil {
		glog.Error(err)
		return err
	}

	wipefsCmd := []string{
		"wipefs",
		"-a",
	}
	execCmd := &cmd.ExecOptions{
		StreamOptions: cmd.StreamOptions{
			PodName:   pod.Name,
			Namespace: pod.Namespace,
			In:        os.Stdin,
			Out:       os.Stdout,
			Err:       os.Stderr,
		},
		Executor:  &cmd.DefaultRemoteExecutor{},
		Config:    &configShallowCopy,
		PodClient: kubectlCoreClient,
	}

	for _, device := range devices {
		glog.Infof("Wiping node %s device %s", node.Name, device)
		execCmd.Command = append(wipefsCmd, device)
		if err := execCmd.Run(); err != nil {
			glog.Error(err)
			return err
		}
	}

	return nil
}

func setConfigDefaults(config *rest.Config) error {
	g, err := scheme.Registry.Group("")
	if err != nil {
		return err
	}

	config.APIPath = "/api"
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	if config.GroupVersion == nil || config.GroupVersion.Group != g.GroupVersion.Group {
		gv := g.GroupVersion
		config.GroupVersion = &gv
	}
	config.NegotiatedSerializer = scheme.Codecs

	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}

	return nil
}
