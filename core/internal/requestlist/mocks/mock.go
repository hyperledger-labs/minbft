// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/hyperledger-labs/minbft/core/internal/requestlist (interfaces: List)

// Package mock_requestlist is a generated GoMock package.
package mock_requestlist

import (
	gomock "github.com/golang/mock/gomock"
	messages "github.com/hyperledger-labs/minbft/messages"
	reflect "reflect"
)

// MockList is a mock of List interface
type MockList struct {
	ctrl     *gomock.Controller
	recorder *MockListMockRecorder
}

// MockListMockRecorder is the mock recorder for MockList
type MockListMockRecorder struct {
	mock *MockList
}

// NewMockList creates a new mock instance
func NewMockList(ctrl *gomock.Controller) *MockList {
	mock := &MockList{ctrl: ctrl}
	mock.recorder = &MockListMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockList) EXPECT() *MockListMockRecorder {
	return m.recorder
}

// Add mocks base method
func (m *MockList) Add(arg0 messages.Request) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Add", arg0)
}

// Add indicates an expected call of Add
func (mr *MockListMockRecorder) Add(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockList)(nil).Add), arg0)
}

// All mocks base method
func (m *MockList) All() []messages.Request {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "All")
	ret0, _ := ret[0].([]messages.Request)
	return ret0
}

// All indicates an expected call of All
func (mr *MockListMockRecorder) All() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "All", reflect.TypeOf((*MockList)(nil).All))
}

// Remove mocks base method
func (m *MockList) Remove(arg0 messages.Request) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Remove", arg0)
}

// Remove indicates an expected call of Remove
func (mr *MockListMockRecorder) Remove(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockList)(nil).Remove), arg0)
}
