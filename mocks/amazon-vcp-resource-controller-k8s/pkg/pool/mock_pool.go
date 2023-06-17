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
// Source: github.com/aws/amazon-vpc-resource-controller-k8s/pkg/pool (interfaces: Pool)

// Package mock_pool is a generated GoMock package.
package mock_pool

import (
	reflect "reflect"

	config "github.com/aws/amazon-vpc-resource-controller-k8s/pkg/config"
	pool "github.com/aws/amazon-vpc-resource-controller-k8s/pkg/pool"
	worker "github.com/aws/amazon-vpc-resource-controller-k8s/pkg/worker"
	gomock "github.com/golang/mock/gomock"
)

// MockPool is a mock of Pool interface.
type MockPool struct {
	ctrl     *gomock.Controller
	recorder *MockPoolMockRecorder
}

// MockPoolMockRecorder is the mock recorder for MockPool.
type MockPoolMockRecorder struct {
	mock *MockPool
}

// NewMockPool creates a new mock instance.
func NewMockPool(ctrl *gomock.Controller) *MockPool {
	mock := &MockPool{ctrl: ctrl}
	mock.recorder = &MockPoolMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPool) EXPECT() *MockPoolMockRecorder {
	return m.recorder
}

// AssignResource mocks base method.
func (m *MockPool) AssignResource(arg0 string) (string, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AssignResource", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// AssignResource indicates an expected call of AssignResource.
func (mr *MockPoolMockRecorder) AssignResource(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AssignResource", reflect.TypeOf((*MockPool)(nil).AssignResource), arg0)
}

// FreeResource mocks base method.
func (m *MockPool) FreeResource(arg0, arg1 string) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FreeResource", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FreeResource indicates an expected call of FreeResource.
func (mr *MockPoolMockRecorder) FreeResource(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FreeResource", reflect.TypeOf((*MockPool)(nil).FreeResource), arg0, arg1)
}

// GetAssignedResource mocks base method.
func (m *MockPool) GetAssignedResource(arg0 string) (string, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAssignedResource", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// GetAssignedResource indicates an expected call of GetAssignedResource.
func (mr *MockPoolMockRecorder) GetAssignedResource(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAssignedResource", reflect.TypeOf((*MockPool)(nil).GetAssignedResource), arg0)
}

// Introspect mocks base method.
func (m *MockPool) Introspect() pool.IntrospectResponse {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Introspect")
	ret0, _ := ret[0].(pool.IntrospectResponse)
	return ret0
}

// Introspect indicates an expected call of Introspect.
func (mr *MockPoolMockRecorder) Introspect() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Introspect", reflect.TypeOf((*MockPool)(nil).Introspect))
}

// ProcessCoolDownQueue mocks base method.
func (m *MockPool) ProcessCoolDownQueue() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ProcessCoolDownQueue")
	ret0, _ := ret[0].(bool)
	return ret0
}

// ProcessCoolDownQueue indicates an expected call of ProcessCoolDownQueue.
func (mr *MockPoolMockRecorder) ProcessCoolDownQueue() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProcessCoolDownQueue", reflect.TypeOf((*MockPool)(nil).ProcessCoolDownQueue))
}

// ReSync mocks base method.
func (m *MockPool) ReSync(arg0 []string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ReSync", arg0)
}

// ReSync indicates an expected call of ReSync.
func (mr *MockPoolMockRecorder) ReSync(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReSync", reflect.TypeOf((*MockPool)(nil).ReSync), arg0)
}

// ReconcilePool mocks base method.
func (m *MockPool) ReconcilePool() *worker.WarmPoolJob {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ReconcilePool")
	ret0, _ := ret[0].(*worker.WarmPoolJob)
	return ret0
}

// ReconcilePool indicates an expected call of ReconcilePool.
func (mr *MockPoolMockRecorder) ReconcilePool() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReconcilePool", reflect.TypeOf((*MockPool)(nil).ReconcilePool))
}

// SetToActive mocks base method.
func (m *MockPool) SetToActive(arg0 *config.WarmPoolConfig) *worker.WarmPoolJob {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetToActive", arg0)
	ret0, _ := ret[0].(*worker.WarmPoolJob)
	return ret0
}

// SetToActive indicates an expected call of SetToActive.
func (mr *MockPoolMockRecorder) SetToActive(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetToActive", reflect.TypeOf((*MockPool)(nil).SetToActive), arg0)
}

// SetToDraining mocks base method.
func (m *MockPool) SetToDraining() *worker.WarmPoolJob {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetToDraining")
	ret0, _ := ret[0].(*worker.WarmPoolJob)
	return ret0
}

// SetToDraining indicates an expected call of SetToDraining.
func (mr *MockPoolMockRecorder) SetToDraining() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetToDraining", reflect.TypeOf((*MockPool)(nil).SetToDraining))
}

// UpdatePool mocks base method.
func (m *MockPool) UpdatePool(arg0 *worker.WarmPoolJob, arg1, arg2 bool) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdatePool", arg0, arg1, arg2)
	ret0, _ := ret[0].(bool)
	return ret0
}

// UpdatePool indicates an expected call of UpdatePool.
func (mr *MockPoolMockRecorder) UpdatePool(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePool", reflect.TypeOf((*MockPool)(nil).UpdatePool), arg0, arg1, arg2)
}
