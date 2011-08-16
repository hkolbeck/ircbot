//Copyright 2011 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package ircbot

import (
	"bufio"
	"net"
	"crypto/tls"
	"os"
)

const (
	ReconnectDelay = 5e9
	KeepAliveInterval = 10e9
	CommBufferSize = 16
	nickserv = `NickServ`
)

type Network {
	conn net.TCPConn  //Connection to the irc server 
	io bufio.ReadWriter //Wraps conn's Read/Write operations
	In <-chan *Message //Channel containing messages the bot recieves
	Out chan<- *Message //Channel containing messages to be sent
	disconnect chan int64 //Internal channel to signal a keepalive failure
	keepalive chan int //The bot's PONG command should send a value down this channel
}

func Dial(addr string, port int, nick, pass, domain string, ssl bool) (*Network, os.Error) {
	network, err := dial(addr, port, nick, pass, domain, ssl)

	if err == nil {
		//Spawn a goroutine to restart network if connection dies
		//Will keep retrying forever at intervals of ReconnectDelay nanoseconds
		//This is seriously racey, and could certainly result in lost messages
		go func() {
			for {
				<-network.disconnect
				network.conn.Close()
				
				if 	newNet, err := dial(addr, port, nick, pass, domain, ssl); err != nil {
					//TODO: Loggit
					network.disconnect <- (<-time.After(ReconnectDelay))
					continue
				}
				
				*network = *newNet
			}
				
		}
	}
	
	return network, err
}

func dial(addr string, port int, nick, domain string, ssl bool) (*Network, os.Error) {
	var conn net.Conn
	var err os.Error

	if conn, err = net.Dial("tcp", addr + string(port)); err != nil {
		//TODO: Log error
		return nil, err
	}
	
	if ssl {
		conn = tls.Client(conn, nil) //Let's try a nil config..
	}
	
	network := &Network{conn, bufio.NewReadWriter(conn, conn), false, make(<-chan *Message, RW_BUFF), make(chan<- *Message, RW_BUFF)}

	network.io.WriteString(fmt.Sprintf("USER %s %s %s :%s\n\r", nick, domain, addr, nick))
	network.io.WriteString("NICK " + nick + "\n\r")

	//Check for connection/nick errors - this will consume first line of motd if no error occurs
	if resp, _, err := network.io.ReadLine(); err == nil && regexp.Match(`[^ \n\r]+ 4[0-9][0-9]`, resp) {
		//Nick already taken, try a ghost kill
		network.io.WriteString(fmt.Sprintf("PRIVMSG %s :ghost %s %s\n\r", nickserv, nick, pass)
		if resp, err := network.io.ReadString(); err == nil && strings.Index(resp, `killed`) != -1 {
			//Looks like it worked, try again
			network.io.WriteString("NICK " + nick + "\n\r")
			//Need another check here?
		} else {
			return nil, os.NewError(string(resp))
		}
	} else if err != nil {
		return nil, err
	}
	
	//Consume motd
	for network.io.Buffered() > 0 {
		network.io.ReadLine()
	}
	
	if pass != "" {
		network.io.WriteString("PRIVMSG %s :identify %s\n\r", nickserv, pass)
	}
	
	//Might fail here if pass is wrong, but we'll let the user deal
	
	go network.listen()
	go network.speak()
	go keepAlive()
		
	return network, nil
}

func (self *Network) keepAlive() {
	tick := time.NewTicker(KeepAliveInterval)
	defer tick.Stop()
		
	for network.connected {
		<-tick.C
		network.Out <- &Message{
			Command : "PING",
			Args : []string{nick}
		}
		timeout := time.After(interval + 5e9)
		select {
			<-timeout: network.disconnect <- 1 
			<-network.keepalive: continue
		}
	}
}


func (self *Network) listen() {
}

func (self *Network) speak() {
	defer func() {
		self.connected = false
		self.conn.Close()
	}()
}

