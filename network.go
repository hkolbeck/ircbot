package ircbot

import "net"
import "bufio"
import "os"
import "fmt"
import "log"
import "time"
import "bytes"

var strError = []byte("ERROR")
var strPing = []byte("PING")
var strPong = []byte("PONG")
var strNewline = byte('\n')

type Config struct {
	Address        string
	Port           int
	Secure         bool
	NickName       string
	CommandPrefix  string
	QuitMessage    string
	Channels       [][]string
}

type Network struct {
	In      chan *Message
	Out     chan *Message
	config  *Config
	tcp     net.Conn
	writer  *bufio.Writer
	reader  *bufio.Reader
	verbose bool
}

func NewNetwork(cfg *Config) *Network {
	return &Network{
		config:  cfg,
		In:      make(chan *Message, 16),
		Out:     make(chan *Message, 16),
		verbose: true,
	}
}

func (this *Network) Open() (err os.Error) {
	addr, err := net.ResolveTCPAddr(fmt.Sprintf("%s:%d", this.config.Address, this.config.Port))
	if err != nil {
		return
	}

	this.tcp, err = net.DialTCP("tcp", nil, addr)
	if err != nil {
		return
	}

	this.reader = bufio.NewReader(this.tcp)
	this.writer = bufio.NewWriter(this.tcp)

	go this.inputLoop()
	go this.outputLoop()
	return
}

func (this *Network) Close() {
	this.Out <- &Message{
	Command : "QUIT", 
	Args : []string{this.config.QuitMessage},
	}

	time.Sleep(3e9) // pause for a few seconds to let the quit go through.

	close(this.In)
	close(this.Out)

	this.reader = nil
	this.writer = nil

	if this.tcp != nil {
		err := this.tcp.Close()
		this.tcp = nil
		if err != nil {
			log.Stderrf("E: %s\n", err)
		}
	}
}

func (this *Network) outputLoop() {
	for {
		if this.writer == nil {
			return
		}

		msg := <-this.Out
		
		if msg == nil {
			continue
		}
		
		
		data := msg.Encode()
		
		if num, err := this.writer.Write(data); num < len(data) {
			log.Stderrf("[e] %s\n", err)
			break
		}
		
		if this.verbose {
			// no use in spamming our logs with ping/pong
			if msg.Command != "PONG" {
				log.Stdoutf("[<] %s", data)
			}
		}
		this.writer.Flush()
	}
}

func (this *Network) inputLoop() {
	domain := fmt.Sprintf("www.%s.com", this.config.NickName)
	
	this.Out <- &Message{
	Command : "USER",
	Args : []string{this.config.NickName, domain, "irc.cat.pdx.edu"},
	Trailing : this.config.NickName,
	}

	this.Out <- &Message{
	Command : "NICK",
	Args : []string{this.config.NickName},
	}



	var err os.Error
	var data []byte

	for {
		if this.reader == nil {
			return
		}

		if data, err = this.reader.ReadBytes(strNewline); err != nil {
			if err != os.EOF {
				log.Stderrf("[E] %s\n", err)
			}
			return
		}

		data = data[0 : len(data)-1]

		if bytes.HasPrefix(data, strError) {
			log.Stderrf("[E] %s\n", data[7:len(data)])
			this.Close()
			return
		}

		if data = bytes.TrimSpace(data); len(data) > 0 {
			if this.verbose {
				//don't spam our logs with ping/pong data.
				if !bytes.HasPrefix(data, strPing){
					log.Stdoutf("[>] %s", data)
				}
			}
			this.In <- Decode(data)
		}
	}
}
