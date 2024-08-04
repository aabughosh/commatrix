// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/openshift-kni/commatrix/debug (interfaces: NewDebugPodInterface)
//
// Generated by this command:
//
//	mockgen -destination=new_debug_mock.go -package=debug . NewDebugPodInterface
//

// Package debug is a generated GoMock package.
package debug

import (
	reflect "reflect"

	client "github.com/openshift-kni/commatrix/client"
	gomock "go.uber.org/mock/gomock"
)

// MockNewDebugPodInterface is a mock of NewDebugPodInterface interface.
type MockNewDebugPodInterface struct {
	ctrl     *gomock.Controller
	recorder *MockNewDebugPodInterfaceMockRecorder
}

// MockNewDebugPodInterfaceMockRecorder is the mock recorder for MockNewDebugPodInterface.
type MockNewDebugPodInterfaceMockRecorder struct {
	mock *MockNewDebugPodInterface
}

// NewMockNewDebugPodInterface creates a new mock instance.
func NewMockNewDebugPodInterface(ctrl *gomock.Controller) *MockNewDebugPodInterface {
	mock := &MockNewDebugPodInterface{ctrl: ctrl}
	mock.recorder = &MockNewDebugPodInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNewDebugPodInterface) EXPECT() *MockNewDebugPodInterfaceMockRecorder {
	return m.recorder
}

// New mocks base method.
func (m *MockNewDebugPodInterface) New(arg0 *client.ClientSet, arg1, arg2, arg3 string) (DebugPodInterface, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "New", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(DebugPodInterface)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// New indicates an expected call of New.
func (mr *MockNewDebugPodInterfaceMockRecorder) New(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "New", reflect.TypeOf((*MockNewDebugPodInterface)(nil).New), arg0, arg1, arg2, arg3)
}