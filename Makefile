GV=6
ircbot:
	$(GV)g -o ircbot.$(GV) bot.go network.go message.go
	cp ircbot.$(GV) mcbot
	cp ircbot.$(GV) acm-bot
clean:
	-rm *.$(GV) *~ 2> /dev/null 