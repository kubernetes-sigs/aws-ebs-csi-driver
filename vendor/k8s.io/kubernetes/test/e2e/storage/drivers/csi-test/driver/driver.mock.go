/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package driver is a generated GoMock package, with required copyright
// header added manually.
package driver

import (
	context "context"
	reflect "reflect"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	gomock "github.com/golang/mock/gomock"
)

// MockIdentityServer is a mock of IdentityServer interface
type MockIdentityServer struct {
	ctrl     *gomock.Controller
	recorder *MockIdentityServerMockRecorder
}

// MockIdentityServerMockRecorder is the mock recorder for MockIdentityServer
type MockIdentityServerMockRecorder struct {
	mock *MockIdentityServer
}

// NewMockIdentityServer creates a new mock instance
func NewMockIdentityServer(ctrl *gomock.Controller) *MockIdentityServer {
	mock := &MockIdentityServer{ctrl: ctrl}
	mock.recorder = &MockIdentityServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockIdentityServer) EXPECT() *MockIdentityServerMockRecorder {
	return m.recorder
}

// GetPluginCapabilities mocks base method
func (m *MockIdentityServer) GetPluginCapabilities(arg0 context.Context, arg1 *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	ret := m.ctrl.Call(m, "GetPluginCapabilities", arg0, arg1)
	ret0, _ := ret[0].(*csi.GetPluginCapabilitiesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPluginCapabilities indicates an expected call of GetPluginCapabilities
func (mr *MockIdentityServerMockRecorder) GetPluginCapabilities(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPluginCapabilities", reflect.TypeOf((*MockIdentityServer)(nil).GetPluginCapabilities), arg0, arg1)
}

// GetPluginInfo mocks base method
func (m *MockIdentityServer) GetPluginInfo(arg0 context.Context, arg1 *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	ret := m.ctrl.Call(m, "GetPluginInfo", arg0, arg1)
	ret0, _ := ret[0].(*csi.GetPluginInfoResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetPluginInfo indicates an expected call of GetPluginInfo
func (mr *MockIdentityServerMockRecorder) GetPluginInfo(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPluginInfo", reflect.TypeOf((*MockIdentityServer)(nil).GetPluginInfo), arg0, arg1)
}

// Probe mocks base method
func (m *MockIdentityServer) Probe(arg0 context.Context, arg1 *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	ret := m.ctrl.Call(m, "Probe", arg0, arg1)
	ret0, _ := ret[0].(*csi.ProbeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Probe indicates an expected call of Probe
func (mr *MockIdentityServerMockRecorder) Probe(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Probe", reflect.TypeOf((*MockIdentityServer)(nil).Probe), arg0, arg1)
}

// MockControllerServer is a mock of ControllerServer interface
type MockControllerServer struct {
	ctrl     *gomock.Controller
	recorder *MockControllerServerMockRecorder
}

// MockControllerServerMockRecorder is the mock recorder for MockControllerServer
type MockControllerServerMockRecorder struct {
	mock *MockControllerServer
}

// NewMockControllerServer creates a new mock instance
func NewMockControllerServer(ctrl *gomock.Controller) *MockControllerServer {
	mock := &MockControllerServer{ctrl: ctrl}
	mock.recorder = &MockControllerServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockControllerServer) EXPECT() *MockControllerServerMockRecorder {
	return m.recorder
}

// ControllerExpandVolume mocks base method
func (m *MockControllerServer) ControllerExpandVolume(arg0 context.Context, arg1 *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	ret := m.ctrl.Call(m, "ControllerExpandVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.ControllerExpandVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ControllerExpandVolume indicates an expected call of ControllerExpandVolume
func (mr *MockControllerServerMockRecorder) ControllerExpandVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControllerExpandVolume", reflect.TypeOf((*MockControllerServer)(nil).ControllerExpandVolume), arg0, arg1)
}

// ControllerGetCapabilities mocks base method
func (m *MockControllerServer) ControllerGetCapabilities(arg0 context.Context, arg1 *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	ret := m.ctrl.Call(m, "ControllerGetCapabilities", arg0, arg1)
	ret0, _ := ret[0].(*csi.ControllerGetCapabilitiesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ControllerGetCapabilities indicates an expected call of ControllerGetCapabilities
func (mr *MockControllerServerMockRecorder) ControllerGetCapabilities(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControllerGetCapabilities", reflect.TypeOf((*MockControllerServer)(nil).ControllerGetCapabilities), arg0, arg1)
}

// ControllerPublishVolume mocks base method
func (m *MockControllerServer) ControllerPublishVolume(arg0 context.Context, arg1 *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	ret := m.ctrl.Call(m, "ControllerPublishVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.ControllerPublishVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ControllerPublishVolume indicates an expected call of ControllerPublishVolume
func (mr *MockControllerServerMockRecorder) ControllerPublishVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControllerPublishVolume", reflect.TypeOf((*MockControllerServer)(nil).ControllerPublishVolume), arg0, arg1)
}

// ControllerUnpublishVolume mocks base method
func (m *MockControllerServer) ControllerUnpublishVolume(arg0 context.Context, arg1 *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	ret := m.ctrl.Call(m, "ControllerUnpublishVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.ControllerUnpublishVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ControllerUnpublishVolume indicates an expected call of ControllerUnpublishVolume
func (mr *MockControllerServerMockRecorder) ControllerUnpublishVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControllerUnpublishVolume", reflect.TypeOf((*MockControllerServer)(nil).ControllerUnpublishVolume), arg0, arg1)
}

// CreateSnapshot mocks base method
func (m *MockControllerServer) CreateSnapshot(arg0 context.Context, arg1 *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	ret := m.ctrl.Call(m, "CreateSnapshot", arg0, arg1)
	ret0, _ := ret[0].(*csi.CreateSnapshotResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateSnapshot indicates an expected call of CreateSnapshot
func (mr *MockControllerServerMockRecorder) CreateSnapshot(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSnapshot", reflect.TypeOf((*MockControllerServer)(nil).CreateSnapshot), arg0, arg1)
}

// CreateVolume mocks base method
func (m *MockControllerServer) CreateVolume(arg0 context.Context, arg1 *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	ret := m.ctrl.Call(m, "CreateVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.CreateVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateVolume indicates an expected call of CreateVolume
func (mr *MockControllerServerMockRecorder) CreateVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateVolume", reflect.TypeOf((*MockControllerServer)(nil).CreateVolume), arg0, arg1)
}

// DeleteSnapshot mocks base method
func (m *MockControllerServer) DeleteSnapshot(arg0 context.Context, arg1 *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	ret := m.ctrl.Call(m, "DeleteSnapshot", arg0, arg1)
	ret0, _ := ret[0].(*csi.DeleteSnapshotResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteSnapshot indicates an expected call of DeleteSnapshot
func (mr *MockControllerServerMockRecorder) DeleteSnapshot(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteSnapshot", reflect.TypeOf((*MockControllerServer)(nil).DeleteSnapshot), arg0, arg1)
}

// DeleteVolume mocks base method
func (m *MockControllerServer) DeleteVolume(arg0 context.Context, arg1 *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	ret := m.ctrl.Call(m, "DeleteVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.DeleteVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteVolume indicates an expected call of DeleteVolume
func (mr *MockControllerServerMockRecorder) DeleteVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteVolume", reflect.TypeOf((*MockControllerServer)(nil).DeleteVolume), arg0, arg1)
}

// GetCapacity mocks base method
func (m *MockControllerServer) GetCapacity(arg0 context.Context, arg1 *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	ret := m.ctrl.Call(m, "GetCapacity", arg0, arg1)
	ret0, _ := ret[0].(*csi.GetCapacityResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetCapacity indicates an expected call of GetCapacity
func (mr *MockControllerServerMockRecorder) GetCapacity(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCapacity", reflect.TypeOf((*MockControllerServer)(nil).GetCapacity), arg0, arg1)
}

// ListSnapshots mocks base method
func (m *MockControllerServer) ListSnapshots(arg0 context.Context, arg1 *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	ret := m.ctrl.Call(m, "ListSnapshots", arg0, arg1)
	ret0, _ := ret[0].(*csi.ListSnapshotsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListSnapshots indicates an expected call of ListSnapshots
func (mr *MockControllerServerMockRecorder) ListSnapshots(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListSnapshots", reflect.TypeOf((*MockControllerServer)(nil).ListSnapshots), arg0, arg1)
}

// ListVolumes mocks base method
func (m *MockControllerServer) ListVolumes(arg0 context.Context, arg1 *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	ret := m.ctrl.Call(m, "ListVolumes", arg0, arg1)
	ret0, _ := ret[0].(*csi.ListVolumesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (m *MockControllerServer) ControllerGetVolume(arg0 context.Context, arg1 *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	ret := m.ctrl.Call(m, "ControllerGetVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.ControllerGetVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ControllerGetVolume indicates an expected call of ControllerGetVolume
func (mr *MockControllerServerMockRecorder) ControllerGetVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControllerGetVolume", reflect.TypeOf((*MockControllerServer)(nil).ControllerGetVolume), arg0, arg1)
}

// ListVolumes indicates an expected call of ListVolumes
func (mr *MockControllerServerMockRecorder) ListVolumes(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListVolumes", reflect.TypeOf((*MockControllerServer)(nil).ListVolumes), arg0, arg1)
}

// ValidateVolumeCapabilities mocks base method
func (m *MockControllerServer) ValidateVolumeCapabilities(arg0 context.Context, arg1 *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	ret := m.ctrl.Call(m, "ValidateVolumeCapabilities", arg0, arg1)
	ret0, _ := ret[0].(*csi.ValidateVolumeCapabilitiesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ValidateVolumeCapabilities indicates an expected call of ValidateVolumeCapabilities
func (mr *MockControllerServerMockRecorder) ValidateVolumeCapabilities(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateVolumeCapabilities", reflect.TypeOf((*MockControllerServer)(nil).ValidateVolumeCapabilities), arg0, arg1)
}

// MockNodeServer is a mock of NodeServer interface
type MockNodeServer struct {
	ctrl     *gomock.Controller
	recorder *MockNodeServerMockRecorder
}

// MockNodeServerMockRecorder is the mock recorder for MockNodeServer
type MockNodeServerMockRecorder struct {
	mock *MockNodeServer
}

// NewMockNodeServer creates a new mock instance
func NewMockNodeServer(ctrl *gomock.Controller) *MockNodeServer {
	mock := &MockNodeServer{ctrl: ctrl}
	mock.recorder = &MockNodeServerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockNodeServer) EXPECT() *MockNodeServerMockRecorder {
	return m.recorder
}

// NodeExpandVolume mocks base method
func (m *MockNodeServer) NodeExpandVolume(arg0 context.Context, arg1 *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	ret := m.ctrl.Call(m, "NodeExpandVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodeExpandVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodeExpandVolume indicates an expected call of NodeExpandVolume
func (mr *MockNodeServerMockRecorder) NodeExpandVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeExpandVolume", reflect.TypeOf((*MockNodeServer)(nil).NodeExpandVolume), arg0, arg1)
}

// NodeGetCapabilities mocks base method
func (m *MockNodeServer) NodeGetCapabilities(arg0 context.Context, arg1 *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	ret := m.ctrl.Call(m, "NodeGetCapabilities", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodeGetCapabilitiesResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodeGetCapabilities indicates an expected call of NodeGetCapabilities
func (mr *MockNodeServerMockRecorder) NodeGetCapabilities(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeGetCapabilities", reflect.TypeOf((*MockNodeServer)(nil).NodeGetCapabilities), arg0, arg1)
}

// NodeGetInfo mocks base method
func (m *MockNodeServer) NodeGetInfo(arg0 context.Context, arg1 *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	ret := m.ctrl.Call(m, "NodeGetInfo", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodeGetInfoResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodeGetInfo indicates an expected call of NodeGetInfo
func (mr *MockNodeServerMockRecorder) NodeGetInfo(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeGetInfo", reflect.TypeOf((*MockNodeServer)(nil).NodeGetInfo), arg0, arg1)
}

// NodeGetVolumeStats mocks base method
func (m *MockNodeServer) NodeGetVolumeStats(arg0 context.Context, arg1 *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	ret := m.ctrl.Call(m, "NodeGetVolumeStats", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodeGetVolumeStatsResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodeGetVolumeStats indicates an expected call of NodeGetVolumeStats
func (mr *MockNodeServerMockRecorder) NodeGetVolumeStats(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeGetVolumeStats", reflect.TypeOf((*MockNodeServer)(nil).NodeGetVolumeStats), arg0, arg1)
}

// NodePublishVolume mocks base method
func (m *MockNodeServer) NodePublishVolume(arg0 context.Context, arg1 *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	ret := m.ctrl.Call(m, "NodePublishVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodePublishVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodePublishVolume indicates an expected call of NodePublishVolume
func (mr *MockNodeServerMockRecorder) NodePublishVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodePublishVolume", reflect.TypeOf((*MockNodeServer)(nil).NodePublishVolume), arg0, arg1)
}

// NodeStageVolume mocks base method
func (m *MockNodeServer) NodeStageVolume(arg0 context.Context, arg1 *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	ret := m.ctrl.Call(m, "NodeStageVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodeStageVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodeStageVolume indicates an expected call of NodeStageVolume
func (mr *MockNodeServerMockRecorder) NodeStageVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeStageVolume", reflect.TypeOf((*MockNodeServer)(nil).NodeStageVolume), arg0, arg1)
}

// NodeUnpublishVolume mocks base method
func (m *MockNodeServer) NodeUnpublishVolume(arg0 context.Context, arg1 *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	ret := m.ctrl.Call(m, "NodeUnpublishVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodeUnpublishVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodeUnpublishVolume indicates an expected call of NodeUnpublishVolume
func (mr *MockNodeServerMockRecorder) NodeUnpublishVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeUnpublishVolume", reflect.TypeOf((*MockNodeServer)(nil).NodeUnpublishVolume), arg0, arg1)
}

// NodeUnstageVolume mocks base method
func (m *MockNodeServer) NodeUnstageVolume(arg0 context.Context, arg1 *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	ret := m.ctrl.Call(m, "NodeUnstageVolume", arg0, arg1)
	ret0, _ := ret[0].(*csi.NodeUnstageVolumeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodeUnstageVolume indicates an expected call of NodeUnstageVolume
func (mr *MockNodeServerMockRecorder) NodeUnstageVolume(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeUnstageVolume", reflect.TypeOf((*MockNodeServer)(nil).NodeUnstageVolume), arg0, arg1)
}
