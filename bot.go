package ircbot

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"log"
	"sync"
	"runtime"
)

type Bot struct {
	Nick string
	Actions map[string]func(*Bot, *Message) *Message
	Prefix byte
	networks map[string]*Network
	myPrefix string
}


//Return a bot which stays connected and nothing else
func NewBot(nick string, prefix byte) (bot *Bot) {
	actions := map[string]func(*Bot, *Message) *Message {
		"PING" : pong,
		"JOIN" : join,
		"PASS" : doNothing,
		"NICK" : doNothing,
		"USER" : doNothing,
		"OPER" : doNothing,
		"MODE" : doNothing,
		"SERVICE" : doNothing,
		"QUIT" : doNothing,
		"SQUIT" : doNothing,
		"PART" : doNothing,
		"TOPIC" : doNothing,
		"NAMES" : doNothing,
		"LIST" : doNothing,
		"INVITE" : doNothing,
		"KICK" : doNothing,
		"PRIVMSG" : doNothing,
		"NOTICE" : doNothing,
		"MOTD" : doNothing,
		"LUSERS" : doNothing,
		"VERSION" : doNothing,
		"STATS" : doNothing,
		"LINKS" : doNothing,
		"TIME" : doNothing,
		"CONNECT" : doNothing,
		"TRACE" : doNothing,
		"ADMIN" : doNothing,
		"INFO" : doNothing,
		"SERVLIST" : doNothing,
		"SQUERY" : doNothing,
		"WHO" : doNothing,
		"WHOIS" : doNothing,
		"WHOWAS" : doNothing,
		"KILL" : doNothing,
		"PONG" : doNothing,
		"ERROR" : doNothing,
		"AWAY" : doNothing,
		"REHASH" : doNothing,
		"DIE" : doNothing,
		"RESTART" : doNothing,
		"SUMMON" : doNothing,
		"USERS" : doNothing,
		"WALLOPS" : doNothing,
		"USERHOST" : doNothing,
		"ISON" : doNothing,
	}

	return &Bot{Nick : nick, Prefix : prefix, Actions : actions, networks : make(map[string]*Network, 10)}
}  
func join(bot *Bot, msg *Message) *Message {
	bot.myPrefix = msg.Prefix 
	return nil
}

func (this *Bot) Send(server string, msg *Message) os.Error {
	ntwrk, ok := this.networks[server]
	
	if !ok {
		return os.NewError(fmt.Sprintf("Error: Attempted send to unknown network: %s", server))
	}

	ntwrk.Out <- msg

	return nil
}

func (this *Bot) Connect(server string, port int, joinChans []string) (joined int, err os.Error) {
	cfg := &Config {
	Address : server,
	Port : port,
	NickName : this.Nick,
	QuitMessage : this.Nick,
        }

	ntwrk := NewNetwork(cfg);

	//Connect to server
	if err = ntwrk.Open(); err != nil {
		return 0, err
	}

	//Wait for USER and NICK commands to be received and processed
	for m := range ntwrk.In {
		if m.Command[0] == '3' || m.Command == "MODE" {
			break
		} else if m.Command == "433" {//Replaceme with constant
			return 0, os.NewError("Error: Nickname already in use")
		} else if m.Command[0] == '4' {
			return 0, os.NewError(fmt.Sprintf("Error %s in USER or NICK command", m.Command))
		}
	}

	//Join specified channels
	for _, chn := range joinChans {
		log.Stdoutf("[I] Attempting to join: %s\n", chn)
		ntwrk.Out <- &Message{
		Command : "JOIN",
		Args : []string{chn},
		}

		for m := range ntwrk.In {
			if m.Command[0] == '3' {
				joined++
				break
			} else if m.Command[0] == '4' {
				break
			}
		}
	}

	this.networks[server] = ntwrk

	go this.run(ntwrk)

	return
}

func (this *Bot) run(ntwrk *Network)  {
	defer ntwrk.Close()
	
	for incMsg := range ntwrk.In {

		if replyFactory, ok :=  this.Actions[incMsg.Command]; ok {
			go func(msg *Message) {
				defer RecoverWithTrace()
				
				reply := (replyFactory)(this, msg)
				
				//All messages limited to 512 chars, figure out how long
				//Body of message can be
				maxTrailing := this.getTrailingMaxLength(msg)
				
				//And report error if prefix + command + args is too long for one message
				//Don't report an error if there's no room for a trailing segment, some
				//messages don't require one.  We handle a 0 case further down.
				if maxTrailing < 0 {
					log.Stderrf("[E] Preamble longer than 512 chars, message not sent: %v", msg)
					return
				}
				
				if msg != nil && reply != nil {
					//Newlines in messages may cause issues, break the message on newlines and 
					//send each piece separately
					messages := strings.Split(strings.TrimSpace(reply.Trailing), "\n", -1)
				
					//Theres a message to be sent, but no room to send it, report error
					if maxTrailing == 0 && len(reply.Trailing) > 0{
						log.Stderrf("[E] Preamble leaves no room for message, message not sent: %v", msg)
						return
					}

					for _, m := range messages {
						if len(m) <= maxTrailing { //If message can be sent in one go, do it
							ntwrk.Out <- &Message{reply.Prefix, reply.Command, reply.Args, m}
						} else {//Otherwise break it up into smaller pieces
							for s, e := 0, maxTrailing; s < len(m) - 1; {
								lastBreak := strings.LastIndex(m[s:e], " ") //Try not to split mid word if possible
								
								if lastBreak != -1 && e != len(m){
									e = lastBreak + s
								}
								
								log.Stdoutf("[I] len(m): %v s: %v e: %v", len(m), s, e)
								ntwrk.Out <- &Message{reply.Prefix, reply.Command, reply.Args, m[s:e]}
								s = e
								e = min(e + maxTrailing, len(m))
							}
						}
					}
				}
			}(incMsg)
		}
	}
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func (this *Bot) getTrailingMaxLength(msg *Message) int {
	maxLength := 512 //Defined in spec
	usedLength := 2//For \r\n

	usedLength += len(this.myPrefix) + 2
	
	if (msg.Command != "") {
		usedLength += len(msg.Command) + 1
	}

	if len(msg.Args) > 0 {
		usedLength += 1

		for _, a := range msg.Args {
			usedLength += len(a) + 1
		}		
	}

	return maxLength - usedLength - 2 //for " :"
}


func (this *Bot) SetPrivmsgHandler(handler, other func(string, *Message) string) {

	//Create a regex to match in-channel messages of the form 'botnick: blah'
	//Match the bot's nick, followed by any char not legal in an irc nick, possibly followed by some number of spaces or tabs.
	//Legal chars are, without the escapes below: a-zA-Z0-9[]{}\|^`-_
	regex := regexp.MustCompile(fmt.Sprintf("^%s[^a-zA-Z0-9\\[\\]{}\\\\\\|\\^`\\-_][ \t]*", this.Nick))
	
	this.Actions["PRIVMSG"] =
		func(bot *Bot, msg *Message) *Message {
		
		var query, reply, target string
		
		if msg.Args[0] == this.Nick { //Private message
			target = msg.GetSender()
			query = msg.Trailing[0:]
			reply = handler(query, msg)
		} else if msg.Trailing[0] == this.Prefix { //Message using prefix
			target = msg.Args[0]
			query = msg.Trailing[1:]
			reply = handler(query, msg)
		} else if match := regex.FindStringIndex(msg.Trailing); match != nil { //In channel message addressed to bot
			target = msg.Args[0]
			query = msg.Trailing[match[1]:]
			reply = handler(query, msg)
		} else if other != nil {
			target = msg.Args[0]
			reply = other(msg.Trailing, msg)
		} else {
			return nil
		}

		if reply == "" {
			return nil
		}

		return &Message{
		Command : "PRIVMSG",
		Args : []string{target},
		Trailing : reply,
		}
	}
}

//Default method invoked for "PING" messages
//Responds with an appropriate "PONG"
func pong(bot *Bot, msg *Message) *Message {
	return &Message{
	Command : "PONG",
	Args : []string{bot.Nick},
	}
}

func doNothing(bot *Bot, msg *Message) *Message {
	return nil
}


func RecoverWithTrace() {
	if x := recover(); x != nil {
		log.Stderrf("[EEE] Runtime Panic caught: %v\n", x)
		
		var btSync sync.Mutex
		btSync.Lock()
		defer btSync.Unlock()
		
		i := 1
		
		for {
			
			pc, file, line, ok := runtime.Caller(i)
			
			if !ok {
				return
			}
			
			f := runtime.FuncForPC(pc)
			log.Stderrf("[EEE]---> %d(%s): %s:%d\n", i-1, f.Name(), file, line)
			i++
		}
		
	}
}
