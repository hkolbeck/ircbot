package main

import (
	"json"
	"io/ioutil"
	"os"
	"log"
	"./ircbot"
)

type botConfig struct {
	BotName string
	AttnChar byte
	IdentPW string

	Server string
	Port int

	Channels []string

	Version string
	SourceLoc string

	Help map[string]string
	Trusted map[string]bool
	TitleWhitelist map[string]bool
}

func parseConfig(filename string) (*botConfig, os.Error){
	raw, err := ioutil.ReadFile(filename)

	if err != nil {
		return nil, err
	}

	c := &botConfig{}

	err = json.Unmarshal(raw, c)

	if err != nil {
		return nil, err
	}

	return c, nil
}

func initParseConfig() {
	var err os.Error

	config, err = parseConfig(configPath)

	if err != nil {
		log.Stderr("[E] Failed to parse config file, exiting")
		os.Exit(1)
	}

	if config.BotName == "" || config.Server == "" {
		log.Stderrf("[E] Malformed config file or missing item,\n%#v\n exiting", config)
		os.Exit(1)
	}

	helpList = "I understand the following commands: "

	for k := range config.Help {
		helpList += k + ", "
	}

	helpList += "source"	
}

func reparseConfig(bot *ircbot.Bot) bool {
	c, err := parseConfig(configPath)

	if err != nil {
		return false
	}

	err = bot.Send(c.Server, &ircbot.Message{
	Command : "NICK",
	Args : []string{c.BotName},
	})

	if err != nil {
		return false
	}

	if c.IdentPW != "" {
		bot.Send(c.Server, &ircbot.Message{
		Command : "PRIVMSG",
		Args : []string{"NickServ"},
		Trailing : "identify " + c.IdentPW,
		})
	}

	for _, channel := range c.Channels {
		bot.Send(c.Server, &ircbot.Message{
		Command : "JOIN",
		Args : []string{channel},
		})
	}

	helpList = "I understand the following commands: "

	for k := range c.Help {
		helpList += k + ", "
	}
	helpList += "source"	

	config = c
	return true
}
