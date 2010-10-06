GV=6
ircbot:
	$(GV)g -o ircbot.$(GV) bot.go network.go message.go

install: ircbot
	mv ircbot.$(GV) $(GOROOT)/pkg/$(GOOS)_$(GOARCH)/ircbot.a

clean:
	-rm *.$(GV) *~ 2> /dev/null 