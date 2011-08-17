//Copyright 2011 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package ircbot

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"regexp"
	"runtime"
	"time"
	"crypto/tls"
)

const (
	//Times in nanoseconds
	ReconnectDelay = 5e9
	KeepAliveInterval = 10e9
	PingTimeout = 5e9
	ReadTimeout = 5e9
	
	CommBufferSize = 16
	nickserv = `NickServ`
)

type Network struct {
	conn net.Conn  //Connection to the irc server 
	io *bufio.ReadWriter //Wraps conn's Read/Write operations
	In chan *Message //Channel containing messages the bot recieves
	Out chan *Message //Channel containing messages to be sent
	disconnect chan int64 //Internal channel to signal a keepalive failure
	keepalive chan int //The bot's PONG command should send a value down this channel
	running bool //Flag polled by goroutines to determine if the should continue running
}

func Dial(addr string, port int, nick, pass, domain string, ssl bool) (*Network, os.Error) {
	network, err := dial(addr, port, nick, pass, domain, ssl)

	if err == nil {
		//Spawn a goroutine to restart network if connection dies
		//Will keep retrying forever at intervals of ReconnectDelay nanoseconds
		//This is seriously racey, and could certainly result in lost messages
		go func() {
			for network.running {
				<-network.disconnect
				network.conn.Close()
				
				if newNet, err := dial(addr, port, nick, pass, domain, ssl); err != nil {
					//TODO: Loggit
					network.disconnect <- (<-time.After(ReconnectDelay))
					continue
				} else {
					*network = *newNet
				}
			}
			network.conn.Close()
				
		}()
	}
	
	return network, err
}

func dial(server string, port int, nick, pass, domain string, ssl bool) (*Network, os.Error) {
	var tcpConn *net.TCPConn
	var conn net.Conn
	var err os.Error
	var resp []byte
	var network *Network
	errRegex, _ := regexp.Compile(`[^ \n\r]+ 4[0-9][0-9]`)

	addr, err := net.ResolveTCPAddr("tcp", server + string(port))
	if err != nil {
		goto Error
	}
	
	if tcpConn, err = net.DialTCP("tcp", nil, addr); err != nil {
		goto Error
	}

	if err = tcpConn.SetKeepAlive(true); err != nil {
		goto Error
	}

	if err = tcpConn.SetReadTimeout(ReadTimeout); err != nil {
		goto Error
	}	

	if ssl {
		conn = tls.Client(conn, nil) //Let's try a nil config..
	} else {
		conn = tcpConn
	}
	
	network = &Network{
	conn : conn, 
	io : bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)), 
	In : make(chan *Message, CommBufferSize), 
	Out : make(chan *Message, CommBufferSize),
	disconnect : make(chan int64),
	keepalive : make(chan int),
	running : true,
	}

	network.io.WriteString(fmt.Sprintf("USER %s %s %s :%s\n\r", nick, domain, addr, nick))
	network.io.WriteString("NICK " + nick + "\n\r")

	

	//Check for connection/nick errors - this will consume first line of motd if no error occurs
	if resp, _, err = network.io.ReadLine(); err == nil && errRegex.Match(resp) {
		//Nick already taken, try a ghost kill
		network.io.WriteString(fmt.Sprintf("PRIVMSG %s :ghost %s %s\n\r", nickserv, nick, pass))
		if resp, _, err = network.io.ReadLine(); err == nil && 
			bytes.Index(resp, []byte(`killed`)) != -1 {
			
			//Looks like it worked, try again
			network.io.WriteString("NICK " + nick + "\n\r")
			//Need another check here?
		} else {
			err = os.NewError(string(resp))
			goto Error
		}
	} else if err != nil {
		goto Error
	}
	
	//Consume motd
	for network.io.Reader.Buffered() > 0 {
		network.io.ReadLine()
	}
	
	if pass != "" {
		network.io.WriteString(fmt.Sprintf("PRIVMSG %s :identify %s\n\r", nickserv, pass))
	}
	
	//Might fail here if pass is wrong, but we'll let the user deal
	
	go network.listen()
	go network.speak()
	go network.keepAlive(nick)
		
	return network, nil
	
Error:
	//TODO: Log error
	return nil, err
}

func (self *Network) HangUp() {
	self.running = false
}

func (self *Network) keepAlive(nick string) {
	tick := time.NewTicker(KeepAliveInterval)
	defer tick.Stop()
		
	for self.running {
		<-tick.C
		self.Out <- &Message{
			Command : "PING",
			Args : []string{nick},
		}
		timeout := time.After(PingTimeout)
		select {
			case <-timeout: self.disconnect <- 1
			case <-self.keepalive: continue
		}
	}
}

func (self *Network) listen() {
	for self.running {
		msg, _, err := self.io.ReadLine()
		if err != nil {
			//TODO: Log failure
			//During disconnection, this could spin, make sure reconnect runs
			runtime.Gosched() 
			continue
		}
		self.In <- Decode(msg)
	}
}

func (self *Network) speak() {
	
	for self.running {
		msg := <-self.Out
		_, err := self.io.Write(msg.Encode())
		
		//If write fails, keep trying
		for err != nil {
			//During disconnection, this could spin, make sure reconnect runs
			runtime.Gosched()
			_, err = self.io.Write(msg.Encode())		
		}
		
	}
}

