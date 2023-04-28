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

// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/aws/amazon-vpc-resource-controller-k8s/pkg/node (interfaces: Node)

// Package mock_node is a generated GoMock package.
package mock_node

import (
	reflect "reflect"

	resource "github.com/aws/amazon-vpc-resource-controller-k8s/pkg/resource"
	gomock "github.com/golang/mock/gomock"
)

// MockNode is a mock of Node interface.
type MockNode struct {
	ctrl     *gomock.Controller
	recorder *MockNodeMockRecorder
}

// MockNodeMockRecorder is the mock recorder for MockNode.
type MockNodeMockRecorder struct {
	mock *MockNode
}

// NewMockNode creates a new mock instance.
func NewMockNode(ctrl *gomock.Controller) *MockNode {
	mock := &MockNode{ctrl: ctrl}
	mock.recorder = &MockNodeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNode) EXPECT() *MockNodeMockRecorder {
	return m.recorder
}

// DeleteResources mocks base method.
func (m *MockNode) DeleteResources(arg0 resource.ResourceManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteResources", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteResources indicates an expected call of DeleteResources.
func (mr *MockNodeMockRecorder) DeleteResources(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteResources", reflect.TypeOf((*MockNode)(nil).DeleteResources), arg0)
}

// GetNodeInstanceID mocks base method.
func (m *MockNode) GetNodeInstanceID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNodeInstanceID")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetNodeInstanceID indicates an expected call of GetNodeInstanceID.
func (mr *MockNodeMockRecorder) GetNodeInstanceID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNodeInstanceID", reflect.TypeOf((*MockNode)(nil).GetNodeInstanceID))
}

// HasInstance mocks base method.
func (m *MockNode) HasInstance() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasInstance")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasInstance indicates an expected call of HasInstance.
func (mr *MockNodeMockRecorder) HasInstance() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasInstance", reflect.TypeOf((*MockNode)(nil).HasInstance))
}

// InitResources mocks base method.
func (m *MockNode) InitResources(arg0 resource.ResourceManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InitResources", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// InitResources indicates an expected call of InitResources.
func (mr *MockNodeMockRecorder) InitResources(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InitResources", reflect.TypeOf((*MockNode)(nil).InitResources), arg0)
}

// IsManaged mocks base method.
func (m *MockNode) IsManaged() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsManaged")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsManaged indicates an expected call of IsManaged.
func (mr *MockNodeMockRecorder) IsManaged() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsManaged", reflect.TypeOf((*MockNode)(nil).IsManaged))
}

// IsNitroInstance mocks base method.
func (m *MockNode) IsNitroInstance() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsNitroInstance")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsNitroInstance indicates an expected call of IsNitroInstance.
func (mr *MockNodeMockRecorder) IsNitroInstance() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsNitroInstance", reflect.TypeOf((*MockNode)(nil).IsNitroInstance))
}

// IsReady mocks base method.
func (m *MockNode) IsReady() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsReady")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsReady indicates an expected call of IsReady.
func (mr *MockNodeMockRecorder) IsReady() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsReady", reflect.TypeOf((*MockNode)(nil).IsReady))
}

// UpdateCustomNetworkingSpecs mocks base method.
func (m *MockNode) UpdateCustomNetworkingSpecs(arg0 string, arg1 []string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "UpdateCustomNetworkingSpecs", arg0, arg1)
}

// UpdateCustomNetworkingSpecs indicates an expected call of UpdateCustomNetworkingSpecs.
func (mr *MockNodeMockRecorder) UpdateCustomNetworkingSpecs(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateCustomNetworkingSpecs", reflect.TypeOf((*MockNode)(nil).UpdateCustomNetworkingSpecs), arg0, arg1)
}

// UpdateResources mocks base method.
func (m *MockNode) UpdateResources(arg0 resource.ResourceManager) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateResources", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateResources indicates an expected call of UpdateResources.
func (mr *MockNodeMockRecorder) UpdateResources(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateResources", reflect.TypeOf((*MockNode)(nil).UpdateResources), arg0)
}
