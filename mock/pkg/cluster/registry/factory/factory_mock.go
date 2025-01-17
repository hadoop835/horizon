// Code generated by MockGen. DO NOT EDIT.
// Source: factory.go

// Package mock_factory is a generated GoMock package.
package mock_factory

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	registry "github.com/horizoncd/horizon/pkg/cluster/registry"
)

// MockFactory is a mock of Factory interface.
type MockFactory struct {
	ctrl     *gomock.Controller
	recorder *MockFactoryMockRecorder
}

// MockFactoryMockRecorder is the mock recorder for MockFactory.
type MockFactoryMockRecorder struct {
	mock *MockFactory
}

// NewMockFactory creates a new mock instance.
func NewMockFactory(ctrl *gomock.Controller) *MockFactory {
	mock := &MockFactory{ctrl: ctrl}
	mock.recorder = &MockFactoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFactory) EXPECT() *MockFactoryMockRecorder {
	return m.recorder
}

// GetRegistryByConfig mocks base method.
func (m *MockFactory) GetRegistryByConfig(ctx context.Context, config *registry.Config) (registry.Registry, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetRegistryByConfig", ctx, config)
	ret0, _ := ret[0].(registry.Registry)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetRegistryByConfig indicates an expected call of GetRegistryByConfig.
func (mr *MockFactoryMockRecorder) GetRegistryByConfig(ctx, config interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetRegistryByConfig", reflect.TypeOf((*MockFactory)(nil).GetRegistryByConfig), ctx, config)
}
