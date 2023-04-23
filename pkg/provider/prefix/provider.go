package prefix

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/api"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/aws/ec2"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/aws/vpc"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/condition"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/config"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/internal/eni"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/pool"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/provider"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/worker"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ipv4PrefixProvider struct {
	// log is the logger initialized with prefix provider details
	log logr.Logger
	// apiWrapper wraps all clients used by the controller
	apiWrapper api.Wrapper
	// workerPool with worker routine to execute asynchronous job on the prefix provider
	workerPool worker.Worker
	// config is the warm pool configuration for the resource prefix
	config *config.WarmPoolConfig
	// lock to allow multiple routines to access the cache concurrently
	lock sync.RWMutex // guards the following
	// instanceResources stores the ENIManager and the resource pool per instance
	instanceProviderAndPool map[string]*ResourceProviderAndPool
	// conditions is used to check which IP allocation mode is enabled
	conditions condition.Conditions
}

// ResourceProviderAndPool contains the instance's ENI manager and the resource pool
type ResourceProviderAndPool struct {
	// lock guards the struct
	lock         sync.RWMutex
	eniManager   eni.ENIManager
	resourcePool pool.Pool
	// prefixToAvailableIPs is mapping from prefix to a list of available IPs
	prefixToAvailableIPs map[string][]string
	// minIPTarget is the minimum number of IPs to be stored in the pool
	minIPTarget int
}

func NewIPv4PrefixProvider(log logr.Logger, apiWrapper api.Wrapper, workerPool worker.Worker,
	resourceConfig config.ResourceConfig, conditions condition.Conditions) provider.ResourceProvider {
	return &ipv4PrefixProvider{
		instanceProviderAndPool: make(map[string]*ResourceProviderAndPool),
		config:                  resourceConfig.WarmPoolConfig,
		log:                     log,
		apiWrapper:              apiWrapper,
		workerPool:              workerPool,
		conditions:              conditions,
	}
}

func (p *ipv4PrefixProvider) InitResource(instance ec2.EC2Instance) error {
	nodeName := instance.Name()
	eniManager := eni.NewENIManager(instance)
	_, presentPrefixes, err := eniManager.InitResources(p.apiWrapper.EC2API)
	if err != nil {
		return err
	}

	runningPods, err := p.apiWrapper.PodAPI.GetRunningPodsOnNode(nodeName)
	if err != nil {
		return err
	}

	prefixToAvailableIPs := make(map[string][]string)

	// usedResourceToGroupMap is a map from prefix to IPs used by running pods
	usedResourceToGroupMap := map[string]string{}

	// Construct list of all possible IPs for each assigned prefix
	for _, prefix := range presentPrefixes {
		ips, err := deconstructIPsFromPrefix(prefix)
		if err != nil {
			return err
		}
		prefixToAvailableIPs[prefix] = ips
	}

	// Map from running pod to its IP
	podToIPMap := map[string]string{}

	for _, pod := range runningPods {
		annotation, present := pod.Annotations[config.ResourceNameIPAddress]
		if !present {
			continue
		}

		// Find the prefix for the running pod's IP
		prefixForIP := ""
		for prefix, availableIPs := range prefixToAvailableIPs {
			for _, candidateIP := range availableIPs {
				// Only mark IP as used if this IP is from the prefix
				if annotation == candidateIP {
					prefixForIP = prefix
					podToIPMap[string(pod.UID)] = annotation
					usedResourceToGroupMap[annotation] = prefix
					break
				}
			}
		}

		// If running pod's IP is not constructed from an assigned prefix on the instance, ignore it
		if prefixForIP == "" {
			p.log.Info("cannot find the prefix for the running pod's IP, secondary IP will be ignored here", "IPv4 address ", annotation)
		} else {
			// Remove used IP from list of available IPs for the prefix
			availableIPs := prefixToAvailableIPs[prefixForIP]
			unUsedIPs := removeElementFromList(availableIPs, annotation)
			prefixToAvailableIPs[prefixForIP] = unUsedIPs
		}
	}

	warmResources := make(map[string][]string, len(presentPrefixes))

	// TODO there may be running pods from secondary IP mode, should subtract them from cap or no?
	nodeCapacity := getCapacity(instance.Type(), instance.Os())

	// Retrieve configuration for prefix delegation from config map and override the default resource config
	resourceConfig := make(map[string]config.ResourceConfig)
	vpcCniConfigMap, err := p.apiWrapper.K8sAPI.GetConfigMap(config.VpcCniConfigMapName, config.KubeSystemNamespace)
	if err == nil {
		resourceConfig = config.LoadResourceConfigFromConfigMap(vpcCniConfigMap)
	} else {
		ctrl.Log.Error(err, "failed to read from config map, will use default resource config")
		resourceConfig = config.LoadResourceConfig()
	}
	p.config = resourceConfig[config.ResourceNameIPAddressFromPrefix].WarmPoolConfig

	resourcePool := pool.NewResourcePool(p.log.WithName("ipv4 prefix resource pool").
		WithValues("node name", instance.Name()), p.config, podToIPMap, usedResourceToGroupMap,
		warmResources, instance.Name(), nodeCapacity)

	p.putInstanceProviderAndPool(nodeName, resourcePool, eniManager, prefixToAvailableIPs, p.config.MinIPTarget)

	p.log.Info("initialized the resource provider for resource IPv4",
		"capacity", nodeCapacity, "node name", nodeName, "instance type",
		instance.Type(), "instance ID", instance.InstanceID(), "warmPoolConfig", p.config, "prefixToAvailableIPs", prefixToAvailableIPs)

	// If PD mode is not enabled, we need to drain the prefix pool
	if !p.conditions.IsWindowsPrefixDelegationEnabled() {
		job := resourcePool.SetToDraining()
		p.SubmitAsyncJob(job)
		p.log.Info("setting the prefix IP pool to draining state")
	} else {
		// Reconcile pool after starting up and submit the async job
		job := resourcePool.ReconcilePool()
		if job.Operations != worker.OperationReconcileNotRequired {
			p.SubmitAsyncJob(job)
		}
	}

	// Submit the async job to periodically process the delete queue
	p.SubmitAsyncJob(worker.NewWarmProcessDeleteQueueJob(nodeName))
	return nil
}

func (p *ipv4PrefixProvider) DeInitResource(instance ec2.EC2Instance) error {
	nodeName := instance.Name()
	p.deleteInstanceProviderAndPool(nodeName)

	return nil
}

func (p *ipv4PrefixProvider) UpdateResourceCapacity(instance ec2.EC2Instance) error {
	resourceProviderAndPool, isPresent := p.getInstanceProviderAndPool(instance.Name())
	if !isPresent {
		p.log.Error(nil, "cannot find the instance provider and pool form the cache", "node-name", instance.Name())
		return nil
	}

	// If secondary IP mode is enabled, then set the current pool state to draining and do not update the capacity as that
	// would be done by secondary IP provider
	if !p.conditions.IsWindowsPrefixDelegationEnabled() {
		job := resourceProviderAndPool.resourcePool.SetToDraining()
		p.log.Info("Secondary IP provider should be active")
		p.SubmitAsyncJob(job)
		return nil
	}

	// Load dynamic configuration for prefix delegation from config map
	vpcCniConfigMap, err := p.apiWrapper.K8sAPI.GetConfigMap(config.VpcCniConfigMapName, config.KubeSystemNamespace)
	resourceConfig := config.LoadResourceConfigFromConfigMap(vpcCniConfigMap)
	ipPrefixResourceConfig, ok := resourceConfig[config.ResourceNameIPAddressFromPrefix]
	if !ok {
		p.log.Error(fmt.Errorf("failed to find resource configuration"), "resourceName", config.ResourceNameIPAddressFromPrefix)
	}

	resourceProviderAndPool.lock.Lock()
	defer resourceProviderAndPool.lock.Unlock()

	resourceProviderAndPool.minIPTarget = ipPrefixResourceConfig.WarmPoolConfig.MinIPTarget

	// Set the prefix provider pool state to active.
	job := resourceProviderAndPool.resourcePool.SetToActive(ipPrefixResourceConfig.WarmPoolConfig)
	p.SubmitAsyncJob(job)

	instanceType := instance.Type()
	instanceName := instance.Name()
	os := instance.Os()
	capacity := getCapacity(instanceType, os)

	// TODO switching from IP mode to PD mode will have secondary IPs from warmpool still in cooldown queue, and nodeCapacity won't include them
	// Advertise capacity of private IPv4 addresses deconstructed from prefixes
	err = p.apiWrapper.K8sAPI.AdvertiseCapacityIfNotSet(instance.Name(), config.ResourceNameIPAddress, capacity)
	if err != nil {
		return err
	}
	p.log.V(1).Info("advertised capacity",
		"instance", instanceName, "instance type", instanceType, "os", os, "capacity", capacity)

	return nil
}

func (p *ipv4PrefixProvider) SubmitAsyncJob(job interface{}) {
	p.workerPool.SubmitJob(job)
}

func (p *ipv4PrefixProvider) ProcessAsyncJob(job interface{}) (ctrl.Result, error) {
	warmPoolJob, isValid := job.(*worker.WarmPoolJob)
	if !isValid {
		return ctrl.Result{}, fmt.Errorf("invalid job type")
	}

	switch warmPoolJob.Operations {
	case worker.OperationCreate:
		p.CreateIPv4PrefixAndUpdatePool(warmPoolJob)
	case worker.OperationDeleted:
		p.DeleteIPv4PrefixAndUpdatePool(warmPoolJob)
	case worker.OperationReSyncPool:
		p.ReSyncPool(warmPoolJob)
	case worker.OperationProcessDeleteQueue:
		return p.ProcessDeleteQueue(warmPoolJob)
	}

	return ctrl.Result{}, nil
}

// CreateIPv4PrefixAndUpdatePool executes the Create IPv4 Prefix workflow by assigning enough IPv4 prefixes to satisfy
// the desired number of IPs required by the warm pool job
func (p *ipv4PrefixProvider) CreateIPv4PrefixAndUpdatePool(job *worker.WarmPoolJob) {
	instanceResource, found := p.getInstanceProviderAndPool(job.NodeName)
	if !found {
		p.log.Error(fmt.Errorf("cannot find the instance provider and pool form the cache"), "node", job.NodeName)
		return
	}

	instanceResource.lock.Lock()
	defer instanceResource.lock.Unlock()

	// Check if available IPs from existing prefixes can satisfy the request; if not, create new prefixes and assign from newly deconstructed IPs
	prefixToAvailableIPs := instanceResource.prefixToAvailableIPs
	allocatedResources := make(map[string][]string)
	count := job.ResourceCount
	for count > 0 {
		for prefix, ips := range prefixToAvailableIPs {
			if count == 0 {
				break
			}
			if ips == nil {
				continue
			}

			// Assign from available IPs
			if count < len(ips) {
				allocatedResources[prefix] = append(allocatedResources[prefix], ips[:count]...)
				ips = ips[count:]
				count -= count
			} else {
				allocatedResources[prefix] = append(allocatedResources[prefix], ips...)
				count -= len(ips)
				ips = nil
			}
			prefixToAvailableIPs[prefix] = ips
		}

		// Check min # prefixes required to satisfy minIPTarget
		minPrefixRequired := int(math.Ceil(float64(instanceResource.minIPTarget) / float64(16)))

		// Not enough IPs from prefixes, need to allocate a new prefix
		if count > 0 || minPrefixRequired > len(prefixToAvailableIPs) {
			p.log.Info("not enough available IPs from existing prefixes, start creating new prefixes")

			prefixCount := int(math.Ceil(float64(count) / float64(16)))
			if prefixCount+len(prefixToAvailableIPs) < minPrefixRequired {
				prefixCount = minPrefixRequired - len(prefixToAvailableIPs)
				p.log.Info("adjusting prefix count to satisfy minIPTarget requirement", "minPrefixRequired", minPrefixRequired,
					"# existing prefixes", len(prefixToAvailableIPs), "prefixCount", prefixCount, " job.ResourceCount", job.ResourceCount)
			}

			prefixes, err := instanceResource.eniManager.CreateIPV4Prefix(prefixCount, p.apiWrapper.EC2API, p.log)
			if err != nil {
				p.log.Error(err, "failed to create all/some of the IPv4 prefixes", "created IPv4 prefixes", prefixes)
				job.Resources = allocatedResources
				instanceResource.prefixToAvailableIPs = prefixToAvailableIPs
				p.updatePoolAndReconcileIfRequired(instanceResource.resourcePool, job, false)
				return
			}

			// Deconstruct newly created prefixes into IPs
			for _, prefix := range prefixes {
				ips, err := deconstructIPsFromPrefix(prefix)
				if err != nil {
					p.log.Error(err, "failed to deconstruct IPs from IPv4 prefix", "prefix", prefix, "error", err)
					job.Resources = allocatedResources
					instanceResource.prefixToAvailableIPs = prefixToAvailableIPs
					p.updatePoolAndReconcileIfRequired(instanceResource.resourcePool, job, false)
					return
				}
				prefixToAvailableIPs[prefix] = ips
			}
		}
	}
	didSucceed := !(count > 0)
	p.log.Info("finishing assigning IPs and prefixes", "didSucceed", didSucceed,
		"allocatedResources", allocatedResources, "prefixToAvailableIPs", prefixToAvailableIPs)

	job.Resources = allocatedResources
	instanceResource.prefixToAvailableIPs = prefixToAvailableIPs
	p.updatePoolAndReconcileIfRequired(instanceResource.resourcePool, job, didSucceed)
}

// DeleteIPv4PrefixAndUpdatePool executes the Delete IPv4 workflow for the list of IPs provided in the warm pool job
func (p *ipv4PrefixProvider) DeleteIPv4PrefixAndUpdatePool(job *worker.WarmPoolJob) {
	instanceResource, found := p.getInstanceProviderAndPool(job.NodeName)
	if !found {
		p.log.Error(fmt.Errorf("cannot find the instance provider and pool form the cache"), "node", job.NodeName)
		return
	}
	instanceResource.lock.Lock()
	defer instanceResource.lock.Unlock()

	prefixToAvailableIPs := instanceResource.prefixToAvailableIPs

	for groupID, resourceIDs := range job.Resources {
		// Add IP back to prefixToAvailableIPs map
		availableIPs, ok := prefixToAvailableIPs[groupID]
		if !ok {
			p.log.Info("the prefix to be updated is not found", "IPv4 prefix", groupID)
			continue
		} else {
			prefixToAvailableIPs[groupID] = append(availableIPs, resourceIDs...)
			p.log.Info("IPs added back to prefixToAvailableIPs", "IPv4 prefix", groupID, "number of available IPs", len(prefixToAvailableIPs[groupID]))
		}
	}

	// Check if prefix has all the available IPs and can be released back to EC2
	var prefixesToRelease []string
	for prefix, availableIPs := range prefixToAvailableIPs {
		if len(availableIPs) == 16 {
			p.log.Info("prefix recycled 16 IPs, can be released back to EC2", "prefix", prefix,
				"prefixToAvailableIPs", prefixToAvailableIPs)
			prefixesToRelease = append(prefixesToRelease, prefix)
		}
	}

	didSucceed := true

	// Keep enough prefixes around to satisfy minIPTarget
	if len(prefixesToRelease) > 0 && instanceResource.minIPTarget > 0 {
		minPrefixRequired := int(math.Ceil(float64(instanceResource.minIPTarget) / float64(16)))
		if len(prefixToAvailableIPs)-len(prefixesToRelease) < minPrefixRequired {
			prefixesToRelease = prefixesToRelease[:len(prefixToAvailableIPs)-minPrefixRequired]
			p.log.Info("ensuring minimumIPTarget requirement", "# existing prefixes", len(prefixToAvailableIPs),
				"minPrefixRequired", minPrefixRequired, "# prefixes to be released", len(prefixesToRelease),
				"prefixesToRelease", prefixesToRelease)
		}
	}

	if len(prefixesToRelease) > 0 {
		failedPrefixes, err := instanceResource.eniManager.DeleteIPV4Prefix(prefixesToRelease, p.apiWrapper.EC2API, p.log)
		if err != nil {
			p.log.Error(err, "failed to delete all/some of the IPv4 prefixes", "failed IPv4 prefixes", failedPrefixes)
			didSucceed = false
		} else {
			for _, prefix := range prefixesToRelease {
				delete(prefixToAvailableIPs, prefix)
				p.log.Info("released prefix to EC2 and deleted key from prefixToAvailableIPs", "prefix", prefix)
			}
		}
	}

	// If failed to delete prefixes, retry the entire job again since none of changes is kept;
	// If it succeeded to delete prefixes, then set job.Resources to nil, and store maps back to instanceResource
	if didSucceed {
		job.Resources = nil
		instanceResource.prefixToAvailableIPs = prefixToAvailableIPs
	}

	p.updatePoolAndReconcileIfRequired(instanceResource.resourcePool, job, didSucceed)
}

func (p *ipv4PrefixProvider) ReSyncPool(job *worker.WarmPoolJob) {
	providerAndPool, found := p.instanceProviderAndPool[job.NodeName]
	if !found {
		p.log.Error(fmt.Errorf("instance provider not found"), "node is not initialized",
			"name", job.NodeName)
		return
	}

	_, prefixes, err := providerAndPool.eniManager.InitResources(p.apiWrapper.EC2API)
	if err != nil {
		p.log.Error(err, "failed to get init resources for the node",
			"name", job.NodeName)
		return
	}

	resources := make(map[string][]string, len(prefixes))
	for _, prefix := range prefixes {
		ips, err := deconstructIPsFromPrefix(prefix)
		if err != nil {
			p.log.Error(err, "failed to deconstruct IPs from prefix", "prefix", prefix)
			return
		}
		resources[prefix] = append(resources[prefix], ips...)
	}

	providerAndPool.resourcePool.ReSync(resources)
}

func (p *ipv4PrefixProvider) ProcessDeleteQueue(job *worker.WarmPoolJob) (ctrl.Result, error) {
	resourceProviderAndPool, isPresent := p.getInstanceProviderAndPool(job.NodeName)
	if !isPresent {
		p.log.Info("forgetting the delete queue processing job", "node", job.NodeName)
		return ctrl.Result{}, nil
	}
	// TODO: For efficiency run only when required in next release
	resourceProviderAndPool.resourcePool.ProcessCoolDownQueue()

	// After the cool down queue is processed check if we need to do reconciliation
	job = resourceProviderAndPool.resourcePool.ReconcilePool()
	if job.Operations != worker.OperationReconcileNotRequired {
		p.SubmitAsyncJob(job)
	}

	// Re-submit the job to execute after cool down period has ended
	return ctrl.Result{Requeue: true, RequeueAfter: config.CoolDownPeriod}, nil
}

// updatePoolAndReconcileIfRequired updates the resource pool and reconcile again and submit a new job if required
func (p *ipv4PrefixProvider) updatePoolAndReconcileIfRequired(resourcePool pool.Pool, job *worker.WarmPoolJob, didSucceed bool) {
	// Update the pool to add the created/failed resource to the warm pool and decrement the pending count
	shouldReconcile := resourcePool.UpdatePool(job, didSucceed)

	if shouldReconcile {
		job := resourcePool.ReconcilePool()
		if job.Operations != worker.OperationReconcileNotRequired {
			p.SubmitAsyncJob(job)
		}
	}
}
func (p *ipv4PrefixProvider) GetPool(nodeName string) (pool.Pool, bool) {
	providerAndPool, exists := p.getInstanceProviderAndPool(nodeName)
	if !exists {
		return nil, false
	}
	return providerAndPool.resourcePool, true
}

func (p *ipv4PrefixProvider) IsInstanceSupported(instance ec2.EC2Instance) bool {
	//TODO add check for Nitro instances
	if instance.Os() == config.OSWindows {
		return true
	}
	return false
}

func (p *ipv4PrefixProvider) Introspect() interface{} {
	p.lock.RLock()
	defer p.lock.RUnlock()

	response := make(map[string]pool.IntrospectResponse)
	for nodeName, resource := range p.instanceProviderAndPool {
		response[nodeName] = resource.resourcePool.Introspect()
	}
	return response
}

func (p *ipv4PrefixProvider) IntrospectNode(node string) interface{} {
	p.lock.RLock()
	defer p.lock.RUnlock()

	resource, found := p.instanceProviderAndPool[node]
	if !found {
		return struct{}{}
	}
	return resource.resourcePool.Introspect()
}

// putInstanceProviderAndPool stores the node's instance provider and pool to the cache
func (p *ipv4PrefixProvider) putInstanceProviderAndPool(nodeName string, resourcePool pool.Pool, manager eni.ENIManager,
	prefixToAvailableIPs map[string][]string, minIPTarget int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	resource := &ResourceProviderAndPool{
		eniManager:           manager,
		resourcePool:         resourcePool,
		prefixToAvailableIPs: prefixToAvailableIPs,
		minIPTarget:          minIPTarget,
	}

	p.instanceProviderAndPool[nodeName] = resource
}

// getInstanceProviderAndPool returns the node's instance provider and pool from the cache
func (p *ipv4PrefixProvider) getInstanceProviderAndPool(nodeName string) (*ResourceProviderAndPool, bool) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	resource, found := p.instanceProviderAndPool[nodeName]
	return resource, found
}

// deleteInstanceProviderAndPool deletes the node's instance provider and pool from the cache
func (p *ipv4PrefixProvider) deleteInstanceProviderAndPool(nodeName string) {
	p.lock.Lock()
	defer p.lock.Unlock()

	delete(p.instanceProviderAndPool, nodeName)
}

// getCapacity returns the capacity for IPv4 addresses deconstructed from IPv4 prefixes based on the instance type and the instance os;
func getCapacity(instanceType string, instanceOs string) int {
	// Assign only 1st ENIs non-primary IP
	limits, found := vpc.Limits[instanceType]
	if !found {
		return 0
	}
	var capacity int
	if instanceOs == config.OSWindows {
		// each /28 IPv4 prefix can be deconstructed into 16 private IPv4 addresses, hence the multiplier of 16
		capacity = (limits.IPv4PerInterface - 1) * 16
	} else {
		capacity = (limits.IPv4PerInterface - 1) * limits.Interface
	}

	return capacity
}

// deconstructIPsFromPrefix deconstructs a /28 IPv4 prefix into a list of /32 IPv4 addresses
func deconstructIPsFromPrefix(prefix string) ([]string, error) {
	var deconstructedIPs []string
	index := strings.Index(prefix, "/")
	if index < 0 {
		return nil, fmt.Errorf("invalid IPv4 prefix %v", prefix)
	}

	addr := strings.Split(prefix[:index], ".")
	networkAddr := addr[0] + "." + addr[1] + "." + addr[2] + "."

	mask, err := strconv.Atoi(prefix[index+1:])
	if err != nil {
		return nil, err
	}
	numOfAddresses := (32 - mask) * 4

	for i := 0; i < numOfAddresses; i++ {
		hostAddr, err := strconv.Atoi(addr[3])
		if err != nil {
			return nil, err
		}
		ipAddr := networkAddr + strconv.Itoa(hostAddr+i) + "/32"
		deconstructedIPs = append(deconstructedIPs, ipAddr)
	}
	return deconstructedIPs, nil
}

// removeElementFromList removes a given string from a list of strings
func removeElementFromList(origList []string, removedIP string) []string {
	var newList []string

	for _, elem := range origList {
		if elem != removedIP {
			newList = append(newList, elem)
		}
	}

	return newList
}
