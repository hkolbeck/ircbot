//Copyright 2010 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package ircbot

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"runtime"
)

type Bot struct {
	Nick string
	Actions map[string]func(*Bot, *Message) *Message
	Attention byte
	network *Network
	myPrefix string
}

//Return a bot which stays connected and nothing else
func NewBot(nick, pass, domain, server string, port int, ssl bool, prefix byte) (*Bot, os.Error) {
	net, err := Dial(server, port, nick, pass, domain, ssl);
	if err != nil {
		return nil, err
	}

	actions := map[string]func(*Bot, *Message) *Message {
		"PING" : pong,
		"JOIN" : join,
		"PONG" : resetTimeout,
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

	bot := &Bot{
	Nick : nick, 
	Attention : prefix, 
	Actions : actions, 
	network : net,
	myPrefix : "",
	}

	go bot.run()
	return bot, nil
}

func (bot *Bot) JoinChannel(channel, pass string) {
	bot.network.Out <- &Message{
	Command : "JOIN",
	Args : []string{channel, pass},
	}
}

func join(bot *Bot, msg *Message) *Message {
	bot.myPrefix = msg.Prefix 
	return nil
}

func resetTimeout(bot *Bot, msg *Message) *Message {
	bot.network.keepalive <- 1
	return nil
}

func (bot *Bot) run()  {
	defer bot.network.HangUp()
	
	for msg := range bot.network.In {
		go bot.dispatch(msg) 
	}
}


//Invokes the message handler and performs pagination
//Of the reply if needed
func (bot *Bot) dispatch(msg *Message) {
	defer RecoverWithTrace()
	var reply *Message

	if msg == nil {
		//TODO: Loggit
		return
	}

	if replyFactory, ok :=  bot.Actions[msg.Command]; ok {
		reply = (replyFactory)(bot, msg)
	} else {
		return //No handler defined for this message type
	}

	if reply == nil {
		return //Nothing more to do if no reply needs sending
	}
	
	
	//All messages limited to 512 chars, figure out how long
	//Body of message can be
	maxTrailing := bot.getTrailingMaxLength(msg)
	
	//Report error if prefix + command + args is too long for one message
	if maxTrailing < 0 {
		//errors.Printf("[E] Preamble longer than 512 chars, message not sent: %v", msg)
		return
	} else if maxTrailing == 0 && len(reply.Trailing) > 0 { //Or if the message has no room for a trailing segment
		//errors.Printf("[E] Preamble leaves no room for message, message not sent: %v", msg)
		return
	}
	
	//Newlines in messages may cause issues, break the message on newlines and 
	//send each piece separately
	messages := strings.Split(strings.TrimSpace(reply.Trailing), "\n")
	
	
	for _, m := range messages {
		if len(m) <= maxTrailing { //If message can be sent in one go, do it
			bot.network.Out <- &Message{
			Prefix : reply.Prefix, 
			Command : reply.Command, 
			Args : reply.Args, 
			Trailing : m,
			}
		} else {//Otherwise break it up into smaller pieces where len <= maxTrailing
			for s, e := 0, maxTrailing; s < len(m) - 1; {
				lastBreak := strings.LastIndex(m[s:e], " ") //Try not to split mid word if possible
				
				if lastBreak != -1 && e != len(m){
					e = lastBreak + s
				}
				
				bot.network.Out <- &Message{
				Prefix : reply.Prefix, 
				Command : reply.Command, 
				Args : reply.Args, 
				Trailing : m[s:e],
				}
				s = e
				e = min(e + maxTrailing, len(m))
			}
		}
	}
}





func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func (bot *Bot) getTrailingMaxLength(msg *Message) int {
	maxLength := 512 //Defined in rfc1459
	usedLength := 2//For \r\n

	usedLength += len(bot.myPrefix) + 2
	
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


func (bot *Bot) SetPrivmsgHandler(handler, other func(string, *Message) string) {

	//Create a regex to match in-channel messages of the form 'botnick: blah'
	//Match the bot's nick, followed by any char not legal in an irc nick, possibly followed by some number of spaces or tabs.
	//Legal chars are, without the escapes below: a-zA-Z0-9[]{}\|^`-_
	regex := regexp.MustCompile(fmt.Sprintf(`^%s[^a-zA-Z0-9\[\]{}\\\|\^\-_][ \t` + "`]*", bot.Nick))
	
	bot.Actions["PRIVMSG"] =
		func(bot *Bot, msg *Message) *Message {
		
		var query, reply, target string
		
		if msg.Args[0] == bot.Nick { //Private message
			target = msg.GetSender()
			query = msg.Trailing[0:]
			reply = handler(query, msg)
		} else if msg.Trailing[0] == bot.Attention { //Message using attention char
			target = msg.Args[0]
			query = msg.Trailing[1:]
			reply = handler(query, msg)
		} else if match := regex.FindStringIndex(msg.Trailing); match != nil { //In channel message addressed to bot
			target = msg.Args[0]
			query = msg.Trailing[match[1]:]
			reply = handler(query, msg)
		} else if other != nil { //Message not directed at bot.
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
		fmt.Printf("[***] Runtime Panic caught: %v\n", x)
		
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
			fmt.Printf("[***]---> %d(%s): %s:%d\n", i-1, f.Name(), file, line)
			i++
		}
		
	}
}
