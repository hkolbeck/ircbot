include $(GOROOT)/src/Make.inc

TARG=cbeck/ircbot
GOFILES=\
	bot.go \
	network.go \
	message.go

include $(GOROOT)/src/Make.pkg

example: 
	$(GC) example.go
	$(LD) -o example-bot example.$(O) 
	rm example.$(O)