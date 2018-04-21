package main

import (
	goflag "flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/gluster/anthill/pkg/client/clientset/versioned"
	anthillinformers "github.com/gluster/anthill/pkg/client/informers/externalversions"
	"github.com/gluster/anthill/pkg/controller"
)

var (
	kubeconfig   string
	apiServerURL string
)

var version = "0.1-alpha"

var rootCmd = &cobra.Command{
	Use:   "anthill",
	Short: "Run the Anthill controller",
	RunE:  run,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf(" Anthill %s\n", version)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig. Only required if out-of-cluster.")
	rootCmd.PersistentFlags().StringVar(&apiServerURL, "server", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	flag.CommandLine.AddGoFlagSet(goflag.CommandLine)
}

func main() {
	goflag.Parse()
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Anthill failed: %+v\n", err)
	}
}

func run(cmd *cobra.Command, args []string) error {
	stopChannel := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, []os.Signal{os.Interrupt, syscall.SIGTERM}...)
	go func() {
		<-c
		close(stopChannel)
		<-c
		os.Exit(1)
	}()

	cfg, err := clientcmd.BuildConfigFromFlags(apiServerURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	anthillClientset, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building anthill clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClientset, time.Second*30)
	anthillInformerFactory := anthillinformers.NewSharedInformerFactory(anthillClientset, time.Second*30)

	ctrl := controller.New(cfg, kubeClientset, anthillClientset, kubeInformerFactory, anthillInformerFactory)

	go kubeInformerFactory.Start(stopChannel)
	go anthillInformerFactory.Start(stopChannel)

	if err = ctrl.Run(2, stopChannel); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}

	return err
}
