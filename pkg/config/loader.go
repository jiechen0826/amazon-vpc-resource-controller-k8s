// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package config

import (
	"strconv"

	v1 "k8s.io/api/core/v1"
)

const (
	// TODO: Should we always do this max retry no matter why it fails
	// such deleted pods will also be retried 5 times, which could be an issue for large pods loads and high churning rate.
	WorkQueueDefaultMaxRetries = 5

	// Default Configuration for Pod ENI resource type
	PodENIDefaultWorker = 7

	// Default Configuration for IPv4 resource type
	IPv4DefaultWorker  = 2
	IPv4DefaultWPSize  = 3
	IPv4DefaultMaxDev  = 1
	IPv4DefaultResSize = 0

	// Default Configuration for IPv4 prefix resource type
	IPv4AddressFromPrefixDefaultWorker  = 2
	IPv4AddressFromPrefixDefaultWPSize  = 3
	IPv4AddressFromPrefixDefaultMaxDev  = 0
	IPv4AddressFromPrefixDefaultResSize = 0

	// EC2 API QPS for user service client
	// Tested: 15 + 20 limits
	// Tested: 15 + 8 limits (not seeing significant degradation from 15+20)
	// Tested: 12 + 8 limits (not seeing significant degradation from 15+8)
	// Larger number seems not make latency better than 12+8
	UserServiceClientQPS      = 12
	UserServiceClientQPSBurst = 8

	// EC2 API QPS for instance service client
	InstanceServiceClientQPS   = 5
	InstanceServiceClientBurst = 7

	// API Server QPS
	DefaultAPIServerQPS   = 10
	DefaultAPIServerBurst = 15
)

// LoadResourceConfig returns the Resource Configuration for all resources managed by the VPC Resource Controller. Currently
// returns the default resource configuration and later can return the configuration from a ConfigMap.
func LoadResourceConfig() map[string]ResourceConfig {
	return getDefaultResourceConfig()
}

func LoadResourceConfigFromConfigMap(vpcCniConfigMap *v1.ConfigMap) map[string]ResourceConfig {
	resourceConfig := getDefaultResourceConfig()

	warmIPTarget, minIPTarget, warmPrefixTarget := ParseWinPDConfigsFromConfigMap(vpcCniConfigMap)

	// If no PD configuration is set in configMap or none is valid, return default resource config
	if warmIPTarget == 0 && minIPTarget == 0 && warmPrefixTarget == 0 {
		return resourceConfig
	}

	desiredSize := IPv4AddressFromPrefixDefaultWPSize
	// Only override the default desiredSize if warmIPTarget is valid (i.e. > 0)
	if warmIPTarget > 0 {
		desiredSize = warmIPTarget
	} else if minIPTarget <= 0 && warmPrefixTarget > 0 {
		// Only set warmPrefixTarget if both warmIPTarget and minIPTarget are 0
		desiredSize = warmPrefixTarget * 16
	}

	customPrefixWarmPoolConfig := WarmPoolConfig{
		DesiredSize:      desiredSize,
		MinIPTarget:      minIPTarget,
		WarmPrefixTarget: warmPrefixTarget,
		MaxDeviation:     IPv4AddressFromPrefixDefaultMaxDev,
		ReservedSize:     IPv4AddressFromPrefixDefaultResSize,
	}

	prefixConfig := resourceConfig[ResourceNameIPAddressFromPrefix]
	prefixConfig.WarmPoolConfig = &customPrefixWarmPoolConfig
	resourceConfig[ResourceNameIPAddressFromPrefix] = prefixConfig
	return resourceConfig
}

func ParseWinPDConfigsFromConfigMap(vpcCniConfigMap *v1.ConfigMap) (warmIPTarget int, minIPTarget int, warmPrefixTarget int) {
	if vpcCniConfigMap.Data == nil {
		return 0, 0, 0
	}

	warmIPTargetStr, foundWarmIP := vpcCniConfigMap.Data[WarmIPTarget]
	minIPTargetStr, foundMinIP := vpcCniConfigMap.Data[MinimumIPTarget]
	warmPrefixTargetStr, foundWarmPrefix := vpcCniConfigMap.Data[WarmPrefixTarget]

	// If no configuration is found, will use default values
	if !foundWarmIP && !foundMinIP && !foundWarmPrefix {
		return 0, 0, 0
	}

	warmIPTarget, minIPTarget, warmPrefixTarget = 0, 0, 0

	if foundWarmIP {
		warmIPTargetInt, err := strconv.Atoi(warmIPTargetStr)
		if err == nil {
			warmIPTarget = warmIPTargetInt
		}
	}
	if foundMinIP {
		minIPTargetInt, err := strconv.Atoi(minIPTargetStr)
		if err == nil {
			minIPTarget = minIPTargetInt
		}
	}
	if foundWarmPrefix {
		warmPrefixTargetInt, err := strconv.Atoi(warmPrefixTargetStr)
		// Only set warm-prefix-target if none of warm-ip-target or minimum-ip-target is set
		if err == nil && warmIPTarget == 0 && minIPTarget == 0 {
			warmPrefixTarget = warmPrefixTargetInt
		}
	}
	return warmIPTarget, minIPTarget, warmPrefixTarget
}

// getDefaultResourceConfig returns the default Resource Configuration.
func getDefaultResourceConfig() map[string]ResourceConfig {

	config := make(map[string]ResourceConfig)

	// Create default configuration for Pod ENI Resource
	podENIConfig := ResourceConfig{
		Name:           ResourceNamePodENI,
		WorkerCount:    PodENIDefaultWorker,
		SupportedOS:    map[string]bool{OSWindows: false, OSLinux: true},
		WarmPoolConfig: nil,
	}
	config[ResourceNamePodENI] = podENIConfig

	// Create default configuration for IPv4 Resource
	ipV4WarmPoolConfig := WarmPoolConfig{
		DesiredSize:  IPv4DefaultWPSize,
		MaxDeviation: IPv4DefaultMaxDev,
		ReservedSize: IPv4DefaultResSize,
	}
	ipV4Config := ResourceConfig{
		Name:           ResourceNameIPAddress,
		WorkerCount:    IPv4DefaultWorker,
		SupportedOS:    map[string]bool{OSWindows: true, OSLinux: false},
		WarmPoolConfig: &ipV4WarmPoolConfig,
	}
	config[ResourceNameIPAddress] = ipV4Config

	// Create default configuration for IPv4 prefix-deconstructed IP resource
	ipV4AddressFromPrefixWarmPoolConfig := WarmPoolConfig{
		DesiredSize:  IPv4AddressFromPrefixDefaultWPSize,
		MaxDeviation: IPv4AddressFromPrefixDefaultMaxDev,
		ReservedSize: IPv4AddressFromPrefixDefaultResSize,
	}
	ipV4PrefixConfig := ResourceConfig{
		Name:           ResourceNameIPAddressFromPrefix,
		WorkerCount:    IPv4AddressFromPrefixDefaultWorker,
		SupportedOS:    map[string]bool{OSWindows: true, OSLinux: false},
		WarmPoolConfig: &ipV4AddressFromPrefixWarmPoolConfig,
	}
	config[ResourceNameIPAddressFromPrefix] = ipV4PrefixConfig

	return config
}
