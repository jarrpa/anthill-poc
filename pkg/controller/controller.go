package controller

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	anthillapi "github.com/gluster/anthill/pkg/apis/anthill/v1alpha1"
	clientset "github.com/gluster/anthill/pkg/client/clientset/versioned"
	anthillscheme "github.com/gluster/anthill/pkg/client/clientset/versioned/scheme"
	informers "github.com/gluster/anthill/pkg/client/informers/externalversions"
	listers "github.com/gluster/anthill/pkg/client/listers/anthill/v1alpha1"
)

// Controller - Anthill Controller
type Controller struct {
	kubeConfig    *restclient.Config
	kubeClientset kubernetes.Interface
	anthillClient clientset.Interface

	glusterLister listers.GlusterClusterLister
	glusterSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder

	workLock *sync.Mutex
}

// WorkItem - Struct to be passed through the workqueue
type WorkItem struct {
	Old interface{}
	New interface{}
}

// New - Create new Anthill Controller
func New(
	kubeConfig *restclient.Config,
	kubeClientset kubernetes.Interface,
	anthillClient clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	anthillInformerFactory informers.SharedInformerFactory) *Controller {

	glusterInformer := anthillInformerFactory.Gluster().V1alpha1().GlusterClusters()

	anthillscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "anthill-controller"})

	controller := &Controller{
		kubeConfig:    kubeConfig,
		kubeClientset: kubeClientset,
		anthillClient: anthillClient,
		glusterLister: glusterInformer.Lister(),
		glusterSynced: glusterInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "GlusterClusters"),
		recorder:      recorder,
		workLock:      &sync.Mutex{},
	}

	glog.Info("Setting up event handlers")
	glusterInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			item := &WorkItem{
				Old: nil,
				New: new,
			}
			controller.workqueue.AddRateLimited(item)
		},
		UpdateFunc: func(old, new interface{}) {
			item := &WorkItem{
				Old: old,
				New: new,
			}
			controller.workqueue.AddRateLimited(item)
		},
		DeleteFunc: func(old interface{}) {
			item := &WorkItem{
				Old: old,
				New: nil,
			}
			controller.workqueue.AddRateLimited(item)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	glog.Info("Starting Anthill controller")

	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.glusterSynced); !ok {
		err := fmt.Errorf("failed to wait for caches to sync")
		glog.Error(err)
		return err
	}

	glog.Info("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.work() {
	}
}

func (c *Controller) work() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		var err error
		err = nil
		var item *WorkItem
		var ok bool
		var old, new, current, oldCluster, newCluster *anthillapi.GlusterCluster

		if item, ok = obj.(*WorkItem); !ok {
			c.workqueue.Forget(item)
			glog.Infof("Expected WorkItem in workqueue but got %#v", item)
			return nil
		}

		if old, ok = item.Old.(*anthillapi.GlusterCluster); old != nil && !ok {
			c.workqueue.Forget(item)
			glog.Infof("Invalid old object: %#v", old)
			return nil
		}

		if new, ok = item.New.(*anthillapi.GlusterCluster); new != nil && !ok {
			c.workqueue.Forget(item)
			glog.Infof("Invalid new object: %#v", new)
			return nil
		}

		if old != nil && new != nil && old.ResourceVersion == new.ResourceVersion {
			// Noop, ignore
			return nil
		}

		if new != nil {
			current, err = c.glusterLister.GlusterClusters(new.Namespace).Get(new.Name)
			if err != nil {
				glog.Error(err)
				return err
			}
			glog.Errorf("Current: %s %s", current.Name, current.ResourceVersion)
			glog.Errorf("New: %s %s", new.Name, new.ResourceVersion)
			if old != nil {
				glog.Errorf("Old: %s %s", old.Name, old.ResourceVersion)
			} else {
				glog.Error("Old: nil")
			}

			// Ignore stale updates
			if current.ResourceVersion > new.ResourceVersion {
				c.workqueue.Forget(item)
				return nil
			}

			// Ignore status updates
			if old != nil && reflect.DeepEqual(new.Spec, old.Spec) {
				glog.Infof("%s equal spec: %s and %s", new.Name, new.ResourceVersion, old.ResourceVersion)
				c.workqueue.Forget(item)
				return nil
			}

			// Only run one update at a time
			c.workLock.Lock()
			defer c.workLock.Unlock()

			if oldCluster, err = c.initGlusterCluster(old); err != nil {
				return err
			}
			if newCluster, err = c.initGlusterCluster(new); err != nil {
				return err
			}

			var status *anthillapi.GlusterClusterStatus
			if new.Status != nil {
				status = new.Status
			} else {
				status = &anthillapi.GlusterClusterStatus{}
			}
			newCluster.Status = status

			glog.Infof("Processing GlusterCluster %s (Version %s)", new.Name, new.ResourceVersion)
			if err = c.updateGlusterCluster(oldCluster, newCluster); err != nil {
				return err
			}

			status.Deployed = true
			if err = c.updateGlusterClusterStatus(new, status); err != nil {
				return err
			}

			glog.Infof("Updated GlusterCluster %s", new.Name)
			c.recorder.Event(new, corev1.EventTypeNormal, "Synced", "GlusterCluster synchronized")
		} else if old != nil && old.Spec.Cascade {
			// TODO
			glog.Infof("Removed GlusterCluster %s", oldCluster.Name)
		}

		c.workqueue.Forget(item)
		glog.Infof("Successfully processed item")
		return nil
	}(obj)

	if err != nil {
		glog.Error(err)
		c.workqueue.AddRateLimited(obj)
	}

	c.workqueue.Done(obj)
	return true
}
