package main

import (
	"fmt"
	clientset "github.com/pefish/k8s-controller-template/pkg/generated/clientset/versioned"
	samplescheme "github.com/pefish/k8s-controller-template/pkg/generated/clientset/versioned/scheme"
	informers "github.com/pefish/k8s-controller-template/pkg/generated/informers/externalversions/pefish/v1alpha1"
	listers "github.com/pefish/k8s-controller-template/pkg/generated/listers/pefish/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

const (
	controllerAgentName = "stu-controller"
	workqueueName       = "Students"
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"

	MessageResourceSynced = "Student synced successfully"
)

type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface  // 用于控制官方定义的组的资源
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface  // 用于控制自己定义的组的资源，比如用来更新资源对象的Status

	studentsLister listers.StudentLister
	studentsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	informer informers.StudentInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme)) // 注册 pefish.k8s.io/v1alpha1 到注册表
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:   kubeclientset,
		sampleclientset: sampleclientset,
		studentsLister:  informer.Lister(),
		studentsSynced:  informer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workqueueName),
		recorder:        recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Foo resources change
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueFoo,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueFoo(new)
		},
	})
	return controller
}

func (c *Controller) enqueueFoo(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Foo controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.studentsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Foo resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(func() {
			for c.processNextWorkItem() {
			}
		}, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get() // 从队列取出对象（是一个 namespace/name 格式的字符串）

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error { // 处理对象
		defer c.workqueue.Done(obj) // 告诉队列这个对象已处理
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, objectName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	studentObject, err := c.studentsLister.Students(namespace).Get(objectName) // 取出这个资源对象
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("studentObject '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}
	fmt.Printf("studentObject: %#v\n", studentObject)
	name := studentObject.Spec.Name
	if name == "" {
		utilruntime.HandleError(fmt.Errorf("%s: name must be specified", key))
		return nil
	}
	// ... 这里可以根据 name 的值做一些事情，这里只打印
	// 每隔一段时间这个资源对象都会被处理（也就是说这个资源对象的值一直被监控着），值一旦发生改变，就可以作出相应处理
	// 比如资源对象中指定了一个deployment的名字，并指定了一个replicas值，作用是控制指定的这个deployment
	// 拿到replicas后与指定的这个deployment的当前的replicas做比较，如果不一样则更新这个deployment
	fmt.Printf("name: %s\n", name)
	school := studentObject.Spec.School
	if school == "" {
		utilruntime.HandleError(fmt.Errorf("%s: name must be specified", key))
		return nil
	}
	fmt.Printf("school: %s\n", school)

	c.recorder.Event(studentObject, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced) // 发送事件
	return nil
}
