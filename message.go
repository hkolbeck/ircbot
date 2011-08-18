//Copyright 2010 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package ircbot

import (
	"bytes"
	"fmt"
	"strings"
)

type Message struct {
	Prefix string
	Command string
	Args []string
	Trailing string
	Ctcp string
}

func (this *Message) Encode() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))

	//Write prefix, if any
	if len(this.Prefix) > 0 {
		buf.WriteByte(':')
		buf.WriteString(this.Prefix)
		buf.WriteByte(' ')
	}
	
	//Write command
	buf.WriteString(this.Command)
	buf.WriteByte(' ')

	//Write args, if any
	for i := range this.Args {
		buf.WriteString(this.Args[i])
		buf.WriteByte(' ')
	}

	//Write any ctcp commands and trailing
	if len(this.Ctcp) > 0 && len(this.Trailing) > 0 {
		buf.WriteString(fmt.Sprintf(":\x01%s %s\x01", this.Ctcp, this.Trailing))
	} else if len(this.Ctcp) == 0 && len(this.Trailing) > 0 {
		buf.WriteString(":" + this.Trailing)
	} else if len(this.Ctcp) > 0 && len(this.Trailing) == 0 {
		buf.WriteString(fmt.Sprintf(":\x01%s\x01", this.Ctcp))
	}

	buf.WriteByte('\n')
	return buf.Bytes()
}

func (this *Message) String() string {
	return string(this.Encode())
}

func Decode(raw []byte) (msg *Message) {
	msg = new(Message)
	raw = bytes.TrimLeft(raw, " ")

	if len(raw) <= 0 {
		return nil
	}
	
	//If message has a prefix, pull it off
	if raw[0] == ':' {
		ind := bytes.IndexByte(raw, ' ');
		msg.Prefix = string(raw[1:ind])
		raw = raw[ind + 1:]
		raw = bytes.TrimLeft(raw, " ")
	}

	//If message has <trailing> pull it off
	if msgStart := bytes.IndexByte(raw, ':'); msgStart > -1 {
		trailBytes := raw[msgStart + 1:]
		raw = raw[0:msgStart]

		//Check if the message contains a CTCP command
		if len(trailBytes) > 0 && trailBytes[0] == '\x01' {
			//Find the terminating 0x01 
			trailBytes = trailBytes[1:]
			ctcpEnd := bytes.IndexByte(trailBytes, ' ')
			if ctcpEnd < 0 { //Nothing in the ctcp but the command
				msg.Ctcp = string(trailBytes[ :len(trailBytes) - 1])
			} else {
				msg.Ctcp = string(trailBytes[:ctcpEnd])
				msg.Trailing = string(trailBytes[ctcpEnd + 1:len(trailBytes) - 1])
			}
		} else {
			msg.Trailing = string(trailBytes)
		}
	}

	args := bytes.Fields(raw)
	msg.Command = string(args[0])
	
	msg.Args = make([]string, len(args) - 1)
	for i, v := range args[1:] {
		msg.Args[i] = string(v)
	}

	return
}

func (m *Message) GetSender() string {
	if m.Prefix == "" {
		return ""
	}

	return m.Prefix[0:strings.Index(m.Prefix, "!")]
}
