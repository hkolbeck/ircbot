package ircbot

import (
	"bytes"
	"strings"
)
type Message struct {
	Prefix string
	Command string
	Args []string
	Trailing string
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

	//Write trailing, if any
	if len(this.Trailing) > 0 {
		buf.WriteByte(':')
		buf.WriteString(this.Trailing)
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
		msg.Trailing = string(raw[msgStart + 1:])
		raw = raw[0:msgStart]
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
