package sessions

import (
	"fmt"
)

type request struct {
	reqType requestType
	data    interface{}
}

type requestType int

const (
	requestTypeChatPM requestType = iota
	requestTypeDelete
	requestTypeNew
	requestTypeNotifyAllChat
	requestTypeNotifyAllError
	requestTypeNotifyAllTable
	requestTypeNotifyAllTableGone
	requestTypeNotifyChatListFromTable
	requestTypeNotifyChatServer
	requestTypeNotifyChatServerPM
	requestTypeNotifyChatTyping
	requestTypeNotifyError
	requestTypeNotifyFriends
	requestTypeNotifyGame
	requestTypeNotifyJoined
	requestTypeNotifyNote
	requestTypeNotifySoundLobby
	requestTypeNotifySpectators
	requestTypeNotifyWarning
	requestTypePrint
	requestTypeSetFriend
	requestTypeSetStatus

	requestTypeShutdown
)

func (m *Manager) requestFuncMapInit() {
	m.requestFuncMap[requestTypeChatPM] = m.chatPM
	m.requestFuncMap[requestTypeDelete] = m.delete
	m.requestFuncMap[requestTypeNew] = m.new
	m.requestFuncMap[requestTypeNotifyAllChat] = m.notifyAllChat
	m.requestFuncMap[requestTypeNotifyAllError] = m.notifyAllError
	m.requestFuncMap[requestTypeNotifyAllTable] = m.notifyAllTable
	m.requestFuncMap[requestTypeNotifyAllTableGone] = m.notifyAllTableGone
	m.requestFuncMap[requestTypeNotifyChatListFromTable] = m.notifyChatListFromTable
	m.requestFuncMap[requestTypeNotifyChatServer] = m.notifyChatServer
	m.requestFuncMap[requestTypeNotifyChatServerPM] = m.notifyChatServerPM
	m.requestFuncMap[requestTypeNotifyChatTyping] = m.notifyChatTyping
	m.requestFuncMap[requestTypeNotifyError] = m.notifyError
	m.requestFuncMap[requestTypeNotifyFriends] = m.notifyFriends
	m.requestFuncMap[requestTypeNotifyGame] = m.notifyGame
	m.requestFuncMap[requestTypeNotifyJoined] = m.notifyJoined
	m.requestFuncMap[requestTypeNotifyNote] = m.notifyNote
	m.requestFuncMap[requestTypeNotifySoundLobby] = m.notifySoundLobby
	m.requestFuncMap[requestTypeNotifySpectators] = m.notifySpectators
	m.requestFuncMap[requestTypeNotifyWarning] = m.notifyWarning
	m.requestFuncMap[requestTypePrint] = m.print
	m.requestFuncMap[requestTypeSetStatus] = m.setStatus
	m.requestFuncMap[requestTypeSetFriend] = m.setFriend
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