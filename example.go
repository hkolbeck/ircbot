//Copyright 2010 Cory Kolbeck <ckolbeck@gmail.com>.
//So long as this notice remains in place, you are welcome 
//to do whatever you like to or with this code.  This code is 
//provided 'As-Is' with no warrenty expressed or implied. 
//If you like it, and we happen to meet, buy me a beer sometime

package main

import (
	irc "cbeck/ircbot"
	"fmt"
)

//Implement a very simple bot which will respond to anyone addressing it and join
//any channels it is invited to.

func main() {
	//Create a new bot
	bot, err := irc.NewBot(
		"goirc-bot", //The bot's nick
		"", //The bot's nickserv password (blank for none)
		"www.github.com",  //The bot's domain
		"chat.freenode.org", //IRC server to connect to
		7070, //Remote port to connect on
		true, //Use ssl?
		'!', //Char used to address the bot
	) 

	if err != nil {
		fmt.Println(err)
		return
	}

	//Control how the bot will react to various IRC commands using the bot's
	//Actions map.  By default the only thing the bot will do is respond to PING
	//requests.  Functions must have the signature:
	// `func(bot *irc.Bot, msg *irc.Message) *irc.Message`
	bot.Actions["INVITE"] = join

	//PRIVMSG can be handled as above, or you can use the convenience method below
	//It accepts two functions, the first arguemnt will be called for messages directed
	//to the bot via its attention char, addressing it by name, or by sending it a 
	//private message.
	//The second will be called for all other messages in 
	bot.SetPrivmsgHandler(sayHi, nil)


	//Connect to the server on the given port, and join any channels specified
	//Returns the number of channels joined (which we're ignoring) and an error if any
	bot.JoinChannel("#echo", "")


	//No further work to be done in main, block indefinitely
	select {}
}

//Whether the bot is addressed with its attention char, in a private message, or with example-bot:
//cmd will hold the meaningful part of the message
//msg holds the raw irc message broken into prefix, command, args, and trailing 
func sayHi(cmd string, msg *irc.Message) string {
	return "Hi there, " + msg.GetSender()
}

//Function to join any channels the bot is invited to.
func join(bot *irc.Bot, msg *irc.Message) *irc.Message {
	return &irc.Message{
		Command : "JOIN",
		Args : []string{msg.Trailing},
	}
}