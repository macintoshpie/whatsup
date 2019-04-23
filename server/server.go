package server

import (
	"encoding/gob"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/brown-csci1380/whatsup/whatsup"
)

// Struct for managing registered users and their connections
type currentConns struct {
	users map[string]whatsup.ChatConn
	conns map[whatsup.ChatConn]string
	mutex *sync.Mutex
}

func serverErrorMsg(errString string) whatsup.WhatsUpMsg {
	// helper for putting error messages into a whatsupmsg
	return whatsup.WhatsUpMsg{
		Action:   whatsup.ERROR,
		Body:     errString,
		Username: "SERVER",
	}
}

func usernameValid(username string) bool {
	return !strings.Contains(username, whatsup.MSG_DELIM)
}

func (currConns *currentConns) register(conn whatsup.ChatConn, username string) {
	// register connection with a new user
	currConns.mutex.Lock()
	defer currConns.mutex.Unlock()

	if _, exists := currConns.conns[conn]; exists {
		whatsup.SendMsg(conn, serverErrorMsg("Connection already registered with user"))
	} else if ok := usernameValid(username); !ok {
		whatsup.SendMsg(conn, serverErrorMsg("Username not valid"))
	} else if _, exists := currConns.users[username]; exists {
		whatsup.SendMsg(conn, serverErrorMsg("Username already taken"))
	} else {
		// successfully register user
		currConns.users[username] = conn
		currConns.conns[conn] = username
	}
}

func (currConns *currentConns) deregister(conn whatsup.ChatConn) {
	// remove user from current users
	currConns.mutex.Lock()
	defer currConns.mutex.Unlock()

	username, _ := currConns.conns[conn]
	delete(currConns.users, username)
	delete(currConns.conns, conn)
}

func (currConns *currentConns) sendUserMsg(msg whatsup.WhatsUpMsg, senderConn whatsup.ChatConn) {
	// send message to an active user or send error to sender
	currConns.mutex.Lock()
	defer currConns.mutex.Unlock()

	userConn, ok := currConns.users[msg.Username]
	if !ok {
		whatsup.SendMsg(senderConn, serverErrorMsg("Unable to find requested user"))
	} else {
		// successfully send message to user
		senderName, _ := currConns.conns[senderConn]
		msg.Username = senderName
		whatsup.SendMsg(userConn, msg)
	}
}

func (currConns *currentConns) listUsers(senderConn whatsup.ChatConn) {
	// send list of current users
	currConns.mutex.Lock()
	defer currConns.mutex.Unlock()

	users := make([]string, 0)
	for user := range currConns.users {
		users = append(users, user)
	}
	usersStr := strings.Join(users, "\n")
	whatsup.SendMsg(senderConn, whatsup.WhatsUpMsg{Action: whatsup.LIST, Body: usersStr, Username: "SERVER"})
}

func handleMsg(
	receivedMsg whatsup.WhatsUpMsg,
	senderConn whatsup.ChatConn,
	currConns *currentConns) (connClosed bool) {

	switch receivedMsg.Action {
	case whatsup.CONNECT:
		currConns.register(senderConn, receivedMsg.Username)
		return false
	case whatsup.MSG:
		currConns.sendUserMsg(receivedMsg, senderConn)
		return false
	case whatsup.LIST:
		currConns.listUsers(senderConn)
		return false
	case whatsup.DISCONNECT:
		currConns.deregister(senderConn)
		return true
	default:
		// unknown, send error
		whatsup.SendMsg(senderConn, serverErrorMsg("Invalid request type"))
		return false
	}
}

func handleConnection(conn net.Conn, currConns *currentConns) {
	// create a new chat connection
	thisConn := whatsup.ChatConn{}
	thisConn.Conn = conn
	defer thisConn.Conn.Close()
	thisConn.Enc = gob.NewEncoder(conn)
	thisConn.Dec = gob.NewDecoder(conn)

	for {
		// continue to receive and process messages until connection is closed
		receivedMsg, err := whatsup.RecvMsg(thisConn)
		if err != nil {
			if err.Error() != "EOF" {
				fmt.Printf("Error receiving message: %v\n", err)
			}
			currConns.deregister(thisConn)
			return
		}

		switch receivedMsg.Action {
		case whatsup.CONNECT:
			currConns.register(thisConn, receivedMsg.Username)
		case whatsup.MSG:
			currConns.sendUserMsg(receivedMsg, thisConn)
		case whatsup.LIST:
			currConns.listUsers(thisConn)
		case whatsup.DISCONNECT:
			currConns.deregister(thisConn)
			return
		default:
			// unknown action, send error
			whatsup.SendMsg(thisConn, serverErrorMsg("Unknown request action"))
		}
	}
}

func Start() {
	listen, port, err := whatsup.OpenListener()
	fmt.Printf("Listening on port %d\n", port)

	if err != nil {
		fmt.Println(err)
		return
	}

	currConns := currentConns{}
	currConns.mutex = &sync.Mutex{}
	currConns.users = make(map[string]whatsup.ChatConn)
	currConns.conns = make(map[whatsup.ChatConn]string)

	for {
		conn, err := listen.Accept() // this blocks until connection or error
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(conn, &currConns)
	}
}
