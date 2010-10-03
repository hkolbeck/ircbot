GV=6
EXE=acm-bot
TEST=test-bot

build: clean ircbot comp
	$(GV)l -o $(EXE) main.$(GV)
	-rm main.6

test: clean ircbot comp
	$(GV)l -o $(TEST) main.$(GV)
	-pkill -9 $(TEST)
	./$(TEST)

ircbot:
	$(GV)g -o ircbot.$(GV) bot.go network.go message.go

comp:
	$(GV)g main.go config.go xmlstructs.go

clean:
	-rm *.$(GV) *~  2> /dev/null 