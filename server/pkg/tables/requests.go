package tables

import (
	"fmt"
)

type request struct {
	reqType requestType
	data    interface{}
}

type requestType int

const (
	requestTypeDisconnectUser requestType = iota
	requestTypeGetTable
	requestTypeGetTables
	requestTypeGetUserTables
	requestTypeJoin
	requestTypeLeave
	requestTypeNewReplay
	requestTypeNewTable
	requestTypePrint
	requestTypeSpectate
	requestTypeUnspectate

	requestTypeShutdown
)

func (m *Manager) requestFuncMapInit() {
	m.requestFuncMap[requestTypeDisconnectUser] = m.disconnectUser
	m.requestFuncMap[requestTypeGetTable] = m.getTable
	m.requestFuncMap[requestTypeGetTables] = m.getTables
	m.requestFuncMap[requestTypeGetUserTables] = m.getUserTables
	m.requestFuncMap[requestTypeJoin] = m.join
	m.requestFuncMap[requestTypeLeave] = m.leave
	m.requestFuncMap[requestTypeNewReplay] = m.newReplay
	m.requestFuncMap[requestTypeNewTable] = m.newTable
	m.requestFuncMap[requestTypePrint] = m.print
	m.requestFuncMap[requestTypeSpectate] = m.spectate
	m.requestFuncMap[requestTypeUnspectate] = m.unspectate
}

// ListenForRequests will block until messages are sent on the request channel.
// It is meant to be run in a new goroutine.
func (m *Manager) ListenForRequests() {
	m.requestsWaitGroup.Add(1)
	defer m.requestsWaitGroup.Done()

	for {
		req := <-m.requests

		if req.reqType == requestTypeShutdown {
			break
		}

		if requestFunc, ok := m.requestFuncMap[req.reqType]; ok {
			requestFunc(req.data)
		} else {
			m.logger.Errorf(
				"The %v manager received an invalid request type of: %v",
				m.name,
				req.reqType,
			)
		}
	}
}

func (m *Manager) newRequest(reqType requestType, data interface{}) error {
	if m.requestsClosed.IsSet() {
		return fmt.Errorf("%v manager is closed to new requests", m.name)
	}

	m.requests <- &request{
		reqType: reqType,
		data:    data,
	}

	return nil
}