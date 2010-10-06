include $(GOROOT)/src/Make.inc

TARG=cbeck/ircbot
GOFILES=\
	bot.go \
	network.go \
	message.go

include $(GOROOT)/src/Make.pkg