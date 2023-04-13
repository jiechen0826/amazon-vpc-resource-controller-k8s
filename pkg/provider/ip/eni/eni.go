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

package eni

import (
	"fmt"
	"strings"
	"sync"

	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/aws/ec2"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/aws/ec2/api"
	"github.com/aws/amazon-vpc-resource-controller-k8s/pkg/aws/vpc"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/go-logr/logr"
)

var (
	ENIDescription = "aws-k8s-eni"
)

type eniManager struct {
	// instance is the pointer to the instance details
	instance ec2.EC2Instance
	// lock to prevent multiple routines concurrently accessing the eni for same node
	lock sync.Mutex // lock guards the following resources
	// attachedENIs is the list of ENIs attached to the instance
	attachedENIs []*eni
	// ipToENIMap is the map from ip to the ENI that it belongs to
	ipToENIMap map[string]*eni
	// ipToENIMap is the map from prefix to the ENI that it belongs to
	prefixToENIMap map[string]*eni
}

// eniDetails stores the eniID along with the number of new IPs that can be assigned form it
type eni struct {
	eniID             string
	remainingCapacity int
}

type ENIManager interface {
	InitResources(ec2APIHelper api.EC2APIHelper) ([]string, []string, error)
	CreateIPV4Address(required int, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error)
	DeleteIPV4Address(ipList []string, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error)
	CreateIPV4Prefix(required int, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error)
	DeleteIPV4Prefix(prefixList []string, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error)
}

// NewENIManager returns a new ENI Manager
func NewENIManager(instance ec2.EC2Instance) *eniManager {
	return &eniManager{
		ipToENIMap:     map[string]*eni{},
		prefixToENIMap: map[string]*eni{},
		instance:       instance,
	}
}

// InitResources loads the list of ENIs, IPs and prefixes, associated with the instance
func (e *eniManager) InitResources(ec2APIHelper api.EC2APIHelper) ([]string, []string, error) {

	nwInterfaces, err := ec2APIHelper.GetInstanceNetworkInterface(aws.String(e.instance.InstanceID()))
	if err != nil {
		return nil, nil, err
	}

	limits, found := vpc.Limits[e.instance.Type()]
	if !found {
		return nil, nil, fmt.Errorf("unsupported instance type")
	}

	ipLimit := limits.IPv4PerInterface
	var availIPs []string
	var availPrefixes []string
	for _, nwInterface := range nwInterfaces {
		if nwInterface.PrivateIpAddresses != nil {
			eni := &eni{
				remainingCapacity: ipLimit,
				eniID:             *nwInterface.NetworkInterfaceId,
			}
			for _, ip := range nwInterface.PrivateIpAddresses {
				if *ip.Primary != true {
					availIPs = append(availIPs, *ip.PrivateIpAddress)
					e.ipToENIMap[*ip.PrivateIpAddress] = eni
				}
				eni.remainingCapacity--
			}
			if nwInterface.Ipv4Prefixes != nil {
				for _, prefix := range nwInterface.Ipv4Prefixes {
					availPrefixes = append(availPrefixes, *prefix.Ipv4Prefix)
					e.prefixToENIMap[*prefix.Ipv4Prefix] = eni
					eni.remainingCapacity--
				}
			}
			e.attachedENIs = append(e.attachedENIs, eni)
		}
	}

	return e.addSubnetMaskToIPSlice(availIPs), availPrefixes, nil
}

// CreateIPV4Address creates IPv4 address and returns the list of assigned IPs along with the error if not all the required
// IPs were assigned
func (e *eniManager) CreateIPV4Address(required int, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	var assignedIPv4Address []string
	log = log.WithValues("node name", e.instance.Name())

	// Loop till we reach the last available ENI and list of assigned IPv4 addresses is less than the required IPv4 addresses
	for index := 0; index < len(e.attachedENIs) && len(assignedIPv4Address) < required; index++ {
		remainingCapacity := e.attachedENIs[index].remainingCapacity
		if remainingCapacity > 0 {
			canAssign := 0
			// Number of IPs wanted is the number of IPs required minus the number of IPs assigned till now.
			want := required - len(assignedIPv4Address)
			// Cannot fulfil the entire request using this ENI, allocate whatever the ENI can assign
			if remainingCapacity < want {
				canAssign = remainingCapacity
			} else {
				canAssign = want
			}
			// Assign the IPv4 Addresses from this ENI
			assignedIPs, err := ec2APIHelper.AssignIPv4AddressesAndWaitTillReady(e.attachedENIs[index].eniID, canAssign)
			if err != nil && len(assignedIPs) == 0 {
				// Return the list of IPs that were actually created along with the error
				return assignedIPs, err
			} else if err != nil {
				// Just log and continue processing the assigned IPs
				log.Error(err, "failed to assign all the requested IPs",
					"requested", want, "got", len(assignedIPs))
			}
			// Update the remaining capacity
			e.attachedENIs[index].remainingCapacity = remainingCapacity - canAssign
			// Append the assigned IPs on this ENI to the list of IPs created across all the ENIs
			assignedIPv4Address = append(assignedIPv4Address, assignedIPs...)
			// Add the mapping from IP to ENI, so that we can easily delete the IP and increment the remaining IP count
			// on the ENI
			for _, ip := range assignedIPs {
				e.ipToENIMap[ip] = e.attachedENIs[index]
			}

			log.Info("assigned IPv4 addresses", "ip", assignedIPs, "eni",
				e.attachedENIs[index].eniID, "want", want, "can provide upto", canAssign)
		}
	}

	// List of secondary IPs supported minus the primary IP
	ipLimit := vpc.Limits[e.instance.Type()].IPv4PerInterface - 1
	eniLimit := vpc.Limits[e.instance.Type()].Interface

	// If the existing ENIs could not assign the required IPs, loop till the new ENIs can assign the required
	// number of IPv4 Addresses
	for len(assignedIPv4Address) < required &&
		len(e.attachedENIs) < eniLimit {
		deviceIndex, err := e.instance.GetHighestUnusedDeviceIndex()
		if err != nil {
			// TODO: Refresh device index for linux nodes only
			return assignedIPv4Address, err
		}
		want := required - len(assignedIPv4Address)
		if want > ipLimit {
			want = ipLimit
		}
		nwInterface, err := ec2APIHelper.CreateAndAttachNetworkInterface(aws.String(e.instance.InstanceID()),
			aws.String(e.instance.SubnetID()), e.instance.InstanceSecurityGroup(), nil, aws.Int64(deviceIndex),
			&ENIDescription, nil, want, 0)
		if err != nil {
			// TODO: Check if any clean up is required here for linux nodes only?
			return assignedIPv4Address, err
		}
		eni := &eni{
			remainingCapacity: ipLimit - want,
			eniID:             *nwInterface.NetworkInterfaceId,
		}
		e.attachedENIs = append(e.attachedENIs, eni)
		for _, assignedIP := range nwInterface.PrivateIpAddresses {
			if !*assignedIP.Primary {
				assignedIPv4Address = append(assignedIPv4Address, *assignedIP.PrivateIpAddress)
				// Also add the mapping from IP to ENI
				e.ipToENIMap[*assignedIP.PrivateIpAddress] = eni
			}
		}
	}

	var err error
	// This can happen if the subnet doesn't have remaining IPs
	if len(assignedIPv4Address) < required {
		err = fmt.Errorf("not able to create the desired number of IPv4 addresses, required %d, created %d",
			required, len(assignedIPv4Address))
	}

	return e.addSubnetMaskToIPSlice(assignedIPv4Address), err
}

// DeleteIPV4Address deletes the list of IPv4 addresses and returns the list of IPs that failed to delete along with the
// error
func (e *eniManager) DeleteIPV4Address(ipList []string, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	var failedToUnAssign []string
	var errors []error

	log = log.WithValues("node name", e.instance.Name())
	ipList = e.stripSubnetMaskFromIPSlice(ipList)

	groupedIPs := e.groupIPsPerENI(ipList)
	for eni, ips := range groupedIPs {
		err := ec2APIHelper.UnassignPrivateIpAddresses(eni.eniID, ips)
		if err != nil {
			errors = append(errors, err)
			log.Info("failed to deleted secondary IPv4 address", "eni", eni.eniID,
				"IPv4 addresses", ips)
			failedToUnAssign = append(failedToUnAssign, ips...)
			continue
		}
		eni.remainingCapacity += len(ips)
		for _, ip := range ips {
			delete(e.ipToENIMap, ip)
		}
		log.Info("deleted secondary IPv4 address", "eni", eni.eniID, "IPv4 addresses", ips)
	}

	ipLimit := vpc.Limits[e.instance.Type()].IPv4PerInterface - 1
	primaryENIID := e.instance.PrimaryNetworkInterfaceID()

	// Clean up ENIs that just have the primary network interface attached to them
	i := 0
	for _, eni := range e.attachedENIs {
		// ENI doesn't have any secondary IP or IP prefix attached to it and is not the primary network interface
		if eni.remainingCapacity == ipLimit && primaryENIID != eni.eniID {
			err := ec2APIHelper.DeleteNetworkInterface(&eni.eniID)
			if err != nil {
				errors = append(errors, err)
				e.attachedENIs[i] = eni
				i++
				continue
			}
			log.Info("deleted ENI successfully as it has no secondary IP attached",
				"id", eni.eniID)
		} else {
			e.attachedENIs[i] = eni
			i++
		}
	}
	e.attachedENIs = e.attachedENIs[:i]

	if errors != nil && len(errors) > 0 {
		return failedToUnAssign, fmt.Errorf("failed to unassign one or more ip addresses %v", errors)
	}

	return nil, nil
}

// CreateIPV4Prefix creates IPv4 prefix and returns the list of assigned IPs along with the error if not all the required
// IPs were assigned
func (e *eniManager) CreateIPV4Prefix(required int, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	var assignedIPv4Prefixes []string
	log = log.WithValues("node name", e.instance.Name())

	// Loop till we reach the last available ENI and list of assigned IPv4 addresses is less than the required IPv4 addresses
	for index := 0; index < len(e.attachedENIs) && len(assignedIPv4Prefixes) < required; index++ {
		remainingCapacity := e.attachedENIs[index].remainingCapacity
		if remainingCapacity > 0 {
			canAssign := 0
			// Number of IPs wanted is the number of IPs required minus the number of IPs assigned till now.
			want := required - len(assignedIPv4Prefixes)
			// Cannot fulfil the entire request using this ENI, allocate whatever the ENI can assign
			if remainingCapacity < want {
				canAssign = remainingCapacity
			} else {
				canAssign = want
			}
			// Assign the IPv4 Addresses from this ENI
			assignedPrefixes, err := ec2APIHelper.AssignIPv4PrefixesAndWaitTillReady(e.attachedENIs[index].eniID, canAssign)
			if err != nil && len(assignedPrefixes) == 0 {
				// Return the list of IP prefixes that were actually created along with the error
				return assignedPrefixes, err
			} else if err != nil {
				// Just log and continue processing the assigned IP prefixes
				log.Error(err, "failed to assign all the requested IP prefixes",
					"requested", want, "got", len(assignedPrefixes))
			}
			// Update the remaining capacity
			e.attachedENIs[index].remainingCapacity = remainingCapacity - canAssign
			// Append the assigned IP prefixes on this ENI to the list of IP prefixes created across all the ENIs
			assignedIPv4Prefixes = append(assignedIPv4Prefixes, assignedPrefixes...)
			// Add the mapping from IP prefix to ENI, so that we can easily delete the prefix and increment the remaining
			// on the ENI
			for _, prefix := range assignedPrefixes {
				e.prefixToENIMap[prefix] = e.attachedENIs[index]
			}

			log.Info("assigned IPv4 prefixes", "ip prefix", assignedPrefixes, "eni",
				e.attachedENIs[index].eniID, "want", want, "can provide upto", canAssign)
		}
	}

	// List of IPv4 prefixes supported minus the primary IP, then minus the assigned secondary IPs
	ipPrefixLimit := vpc.Limits[e.instance.Type()].IPv4PerInterface - 1
	eniLimit := vpc.Limits[e.instance.Type()].Interface

	// If the existing ENIs could not assign the required IPv4 prefixes, loop till the new ENIs can assign the required
	// number of IPv4 prefixes
	for len(assignedIPv4Prefixes) < required && len(e.attachedENIs) < eniLimit {
		deviceIndex, err := e.instance.GetHighestUnusedDeviceIndex()
		if err != nil {
			// TODO: Refresh device index for linux nodes only
			return assignedIPv4Prefixes, err
		}
		want := required - len(assignedIPv4Prefixes)
		if want > ipPrefixLimit {
			want = ipPrefixLimit
		}

		nwInterface, err := ec2APIHelper.CreateAndAttachNetworkInterface(aws.String(e.instance.InstanceID()),
			aws.String(e.instance.SubnetID()), e.instance.InstanceSecurityGroup(), nil, aws.Int64(deviceIndex),
			&ENIDescription, nil, 0, want)
		if err != nil {
			// TODO: Check if any clean up is required here for linux nodes only?
			return assignedIPv4Prefixes, err
		}
		eni := &eni{
			remainingCapacity: ipPrefixLimit - want,
			eniID:             *nwInterface.NetworkInterfaceId,
		}
		e.attachedENIs = append(e.attachedENIs, eni)
		for _, assignedPrefix := range nwInterface.Ipv4Prefixes {
			assignedIPv4Prefixes = append(assignedIPv4Prefixes, *assignedPrefix.Ipv4Prefix)
			// Also add the mapping from IP to ENI
			e.prefixToENIMap[*assignedPrefix.Ipv4Prefix] = eni
		}
	}

	var err error
	// This can happen if the subnet doesn't have any remaining slots
	if len(assignedIPv4Prefixes) < required {
		err = fmt.Errorf("not able to create the desired number of IPv4 prefixes, required %d, created %d",
			required, len(assignedIPv4Prefixes))
	}

	return assignedIPv4Prefixes, err
}

// DeleteIPV4Prefix deletes the list of IPv4 addresses and returns the list of IPs that failed to delete along with the
// // error
func (e *eniManager) DeleteIPV4Prefix(prefixList []string, ec2APIHelper api.EC2APIHelper, log logr.Logger) ([]string, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	var failedToUnAssign []string
	var errors []error

	log = log.WithValues("node name", e.instance.Name())

	groupedPrefixes, err := e.groupPrefixesPerENI(prefixList)
	if err != nil {
		return nil, err
	}

	for eni, prefixes := range groupedPrefixes {
		err := ec2APIHelper.UnassignIPv4Prefixes(eni.eniID, prefixes)
		if err != nil {
			errors = append(errors, err)
			log.Info("failed to delete IPv4 prefix", "eni", eni.eniID,
				"IPv4 prefixes", prefixes)
			failedToUnAssign = append(failedToUnAssign, prefixes...)
			continue
		}
		eni.remainingCapacity += len(prefixes)
		for _, prefix := range prefixes {
			delete(e.prefixToENIMap, prefix)
		}
		log.Info("deleted IPv4 prefix", "eni", eni.eniID, "IPv4 prefixes", prefixes)
	}

	ipLimit := vpc.Limits[e.instance.Type()].IPv4PerInterface - 1
	primaryENIID := e.instance.PrimaryNetworkInterfaceID()

	// Clean up ENIs that just have the primary network interface attached to them
	i := 0
	for _, eni := range e.attachedENIs {
		// ENI doesn't have any secondary IP attached to it and is not the primary network interface
		if eni.remainingCapacity == ipLimit && primaryENIID != eni.eniID {
			err := ec2APIHelper.DeleteNetworkInterface(&eni.eniID)
			if err != nil {
				errors = append(errors, err)
				e.attachedENIs[i] = eni
				i++
				continue
			}
			log.Info("deleted ENI successfully as it has no secondary IP or IPv4 prefix attached",
				"id", eni.eniID)
		} else {
			e.attachedENIs[i] = eni
			i++
		}
	}
	e.attachedENIs = e.attachedENIs[:i]

	if errors != nil && len(errors) > 0 {
		return failedToUnAssign, fmt.Errorf("failed to unassign one or more IPv4 prefixes %v", errors)
	}

	return nil, nil
}

// groupIPsPerENI groups the IPs to delete per ENI
func (e *eniManager) groupIPsPerENI(deleteList []string) map[*eni][]string {
	toDelete := map[*eni][]string{}
	for _, ip := range deleteList {
		eni := e.ipToENIMap[ip]
		ls := toDelete[eni]
		ls = append(ls, ip)
		toDelete[eni] = ls
	}

	return toDelete
}

// groupPrefixesPerENI groups the IPs to delete per ENI
func (e *eniManager) groupPrefixesPerENI(deleteList []string) (map[*eni][]string, error) {
	toDelete := map[*eni][]string{}
	for _, prefix := range deleteList {
		eni, ok := e.prefixToENIMap[prefix]
		if !ok {
			return nil, fmt.Errorf("prefix %v does not exist or is invalid", prefix)
		}
		ls := toDelete[eni]
		ls = append(ls, prefix)
		toDelete[eni] = ls
	}

	return toDelete, nil
}

func (e *eniManager) addSubnetMaskToIPSlice(ipAddresses []string) []string {
	for i := 0; i < len(ipAddresses); i++ {
		ipAddresses[i] = ipAddresses[i] + "/" + e.instance.SubnetMask()
	}
	return ipAddresses
}

func (e *eniManager) stripSubnetMaskFromIPSlice(ipAddresses []string) []string {
	for i := 0; i < len(ipAddresses); i++ {
		ipAddresses[i] = strings.Split(ipAddresses[i], "/")[0]
	}
	return ipAddresses
}
