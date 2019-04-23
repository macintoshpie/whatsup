package client

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/brown-csci1380/whatsup/whatsup"
)

const USAGE string = "Usage:\n  quit\n  list\n  set-name\n  <username>: <message>"

func getUserInput(input chan whatsup.WhatsUpMsg) {
	// read lines from stdin into input channel
	reader := bufio.NewReader(os.Stdin)
	for {
		userIn, err := reader.ReadString('\n')
		if err != nil {
			close(input)
			return
		}
		userIn = strings.TrimSpace(userIn)
		var msg whatsup.WhatsUpMsg
		switch userIn {
		case "quit":
			// deregister user and disconnect
			msg = whatsup.WhatsUpMsg{Action: whatsup.DISCONNECT}
			input <- msg
			close(input)
			return
		case "list":
			// list server's registered users
			msg = whatsup.WhatsUpMsg{Action: whatsup.LIST}
		case "set-name":
			// request to register name for the connection
			fmt.Print("name: ")
			newName, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("%v\n", err)
				close(input)
				return
			}
			newName = strings.TrimSpace(newName)
			msg = whatsup.WhatsUpMsg{Action: whatsup.CONNECT, Username: newName}
		default:
			// parse as a message
			userInSplit := strings.Split(userIn, whatsup.MSG_DELIM)
			if len(userInSplit) != 2 {
				fmt.Println("Invalid input")
				fmt.Println(USAGE)
				continue
			}
			msg = whatsup.WhatsUpMsg{
				Body:     strings.TrimSpace(userInSplit[1]),
				Username: strings.TrimSpace(userInSplit[0]),
				Action:   whatsup.MSG,
			}
		}
		// put message into queue for sending
		input <- msg
	}
}

func listenForMessages(conn whatsup.ChatConn, msgChn chan whatsup.WhatsUpMsg) {
	// list to connection for messages from server
	for {
		msg, err := whatsup.RecvMsg(conn)
		if err != nil {
			if err.Error() == "EOF" {
				fmt.Println("Server terminated connection")
			} else {
				fmt.Printf("Error receiving message: %v\n", err)
			}
			close(msgChn)
			return
		}
		msgChn <- msg
	}
}

func Start(user string, serverPort string, serverAddr string) {
	// Connect to chat server
	conn, err := whatsup.ServerConnect(user, serverAddr, serverPort)
	if err != nil {
		fmt.Printf("unable to connect to server: %s\n", err)
		return
	}

	userInput := make(chan whatsup.WhatsUpMsg)
	go getUserInput(userInput)
	userMsgs := make(chan whatsup.WhatsUpMsg)
	go listenForMessages(conn, userMsgs)
	for {
		select {
		case userIn, ok := <-userInput:
			if !ok {
				// client quit
				close(userMsgs)
				return
			}
			// send message to server
			whatsup.SendMsg(conn, userIn)
		case userMsg, ok := <-userMsgs:
			if !ok {
				// done processing messages
				close(userInput)
				return
			}
			// print received message
			fmt.Printf("%v - %v: %v\n", userMsg.Action, userMsg.Username, userMsg.Body)
		}
	}
}
