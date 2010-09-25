package main

import (
	"./ircbot"
	"xml"
	"time"
	"fmt"
	"strings"
	"runtime"
	"http"
	"log"
	"os"
	"regexp"
	"io/ioutil"
	"io"
	"flag"
)

var configPath string
var config *botConfig
var helpList string
var bot *ircbot.Bot

func init() {
	flag.StringVar(&configPath, "c", os.Args[0] + ".conf", "config file to use, defaults to <executable name>.conf")
	
	runtime.GOMAXPROCS(32)
}

func main() {
	flag.Parse()
	initParseConfig()

	for {
		session()
		time.Sleep(10e9)
	}
}

func session() {
	defer ircbot.RecoverWithTrace()
	bot = ircbot.NewBot(config.BotName, config.AttnChar)

	bot.Actions["INVITE"] = join
	bot.SetPrivmsgHandler(parseCommand, parseChat)
	_, e := bot.Connect(config.Server, config.Port, config.Channels)

	if e != nil {
		panic(e.String())
	}


	if config.IdentPW != "" {
		bot.Send(config.Server, &ircbot.Message{
		Command : "PRIVMSG",
		Args : []string{"NickServ"},
		Trailing : "identify " + config.IdentPW,
		})
	}

	select {}
}

//Break command into command and args, then call appropriate command
func parseCommand(c string, m *ircbot.Message) string {
	var args []string
	split := strings.Split(strings.TrimSpace(c), " ", 2)
	command := strings.ToLower(split[0])
	if len(split) > 1 {
		args = strings.Split(split[1], " ", -1)
	}

	recordSighting(m)

	switch command {
	case "help" : return help(args)
	case "cal", "calendar" : return getCal(args)
	case "dict", "define" : return dictLookup(args)
	case "version" : return config.Version
	case "seen" : return seen(args, m)
	case "source", config.BotName : return config.SourceLoc
	case "spam" : return spam(c, m)
	case "ignore" : return ignore(args, m)
	case "reconf" : return reconf(m)
	case "" : return ""
	}

	return "Huh?"
}

type sighting struct {
	at *time.Time
	said string
}

var sightings map[string]map[string]*sighting = make(map[string]map[string]*sighting, 100)

func recordSighting(m *ircbot.Message) {
	s, ok := sightings[m.GetSender()];
	
	if !ok {
		s = make(map[string]*sighting)
		sightings[m.GetSender()] = s
	}

	s[m.Args[0]] = &sighting{time.LocalTime(), m.Trailing}
}

func spam(c string, m *ircbot.Message) string {
	msgStart := strings.Index(c, ":")
	if msgStart == -1 {
		return "No message specified, try `?spam <recipients> :<message>`"
	}

	msg := strings.TrimSpace(c[msgStart + 1:])

	if len(msg) == 0 {
		return "Empty message not sent"
	}

	recipients := strings.Split(c[:msgStart], " ", -1)[1:]

	if len(recipients) == 0 {
		return "No recipients specified, try `?spam <recipients> :<message>`"
	}

	for _, r := range recipients {
		_ = bot.Send(config.Server, &ircbot.Message{
		Command : "PRIVMSG",
		Trailing : fmt.Sprintf("%s sez: %s",  m.GetSender(), msg),
		Args : []string{r},
		})
	}

	return "Ok, " + m.GetSender()
}

func ignore(args []string, m *ircbot.Message) string {
	if config.Trusted[m.GetSender()] {
		for _, u := range args {
			if u[0] == '-' {
				config.Ignores[u[1:]] = false, false
			} else if u[0] == '+' {
				config.Ignores[u[1:]] = true
			} else {
				config.Ignores[u] = true
			}
		}
		return "Ok, " + m.GetSender()
	}
	return "I'm afraid I can't do that " + m.GetSender()
}

func reconf(m *ircbot.Message) string {
	if config.Trusted[m.GetSender()] {
		if reparseConfig(bot) {
			return "Reparsed " + configPath + " successfully."
		} else {
			return "Failed to reparse " + configPath + "."
		}
	}
	return "I'm afraid I can't do that " + m.GetSender()
}

var urlRegex *regexp.Regexp = regexp.MustCompile(`http://([a-zA-Z0-9_\-.]+)+(:[0-9]+)?[^ ]*`)

func parseChat(msg string, m *ircbot.Message) (reply string) {
	if config.Ignores[m.GetSender()] {
		return ""
	}

	if matches := urlRegex.FindAllStringSubmatch(msg, -1); matches != nil {
		for _, m := range matches {
			
			response, finalURL, err := http.Get(m[0])
			
			if err != nil {
				log.Stderrf("[E] %s - Fetch failed: %s\n", m[0], err.String())
			} else if finalURL != m[0] || config.TitleWhitelist[m[1]] {
				if t := getTitle(response.Body); t != "" {
					reply += fmt.Sprintf("Title:%s\n", t)
				}
			}
		}
	}

	recordSighting(m)
	
	return
}

var titleRegex *regexp.Regexp = regexp.MustCompile("<title>(.*)</title>")
var multiSpace *regexp.Regexp = regexp.MustCompile("[ \t\n]+")

func getTitle(text io.Reader) string {
	t, err := ioutil.ReadAll(text)

	if err != nil {
		return ""
	}

	m := titleRegex.FindSubmatch(t)

	if m == nil || len(m) < 2{
		return ""
	}

	return multiSpace.ReplaceAllString(" " + string(m[1]), " ")
}

func join(bot *ircbot.Bot, msg *ircbot.Message) *ircbot.Message {
	if config.Trusted[msg.GetSender()] {
		return &ircbot.Message{
		Command : "JOIN",
		Args : []string{msg.Trailing},
		}
	}

	return nil
}

func seen(users []string, msg *ircbot.Message) (reply string) {
	for _, u := range users {
		if s, ok := sightings[u]; ok {
			if cs, ok := s[msg.Args[0]]; ok {
				reply += fmt.Sprintf("%s was last seen %s saying \"%s\"", u, cs.at.String()[0:19], cs.said)
				continue
			}
		}
		reply += fmt.Sprintf("I havn't seen %s\n", u)
	}

	return
}

func help(args []string) (answer string) {
	if args == nil {
		answer = "Queries may be addressed to me in channel with 'acm-bot: <query>' or '?query', or via private message.\n" +
			helpList
	} else if a, ok := config.Help[args[0]]; ok {
		answer = a
	} else {
		answer = fmt.Sprintf("No help available for '%s', try typing '?help' for a list of available commands", args[0]) 
	}

	return
}



func dictLookup(query []string) (reply string) {
	if query == nil {
		return "[This space intentionally left blank]"
	} 
	
	q := strings.Join(query, "+")
	response, _, err := http.Get(fmt.Sprintf("http://jws-champo.ac-toulouse.fr:8080/wordnet-xml/servlet?word=%s&submit=Query", q))
	
	if err != nil {
		log.Stderrf("[E] GET error: %s\n", err.String())
		return fmt.Sprintf("Unable to reach WordNet, try again later\n", q)
	}

	defer response.Body.Close()
	
	wq := &dictWordnet{}
	err = xml.Unmarshal(response.Body, wq)
	
	if err != nil {
		log.Stderrf("[E] XML unmarshal error: %s\n", err.String())
		return fmt.Sprintf("Entry for %s seems to be corrupted\n", q)
	}
	
	qDisp := strings.Join(query, " ")

	for _, p := range wq.Pos {
		for _, s := range p.Category.Sense { 
			reply += fmt.Sprintf("(%s) *%s*: %s\n", p.Name, qDisp, s.Synset.Definition)
		}
	}

	if reply == "" {
		return fmt.Sprintf("No definition found for %s.\n", query[0])
	}
	
	return reply
}

const (
	timeFormat = "01/02/06"
	timeFormatHelp = "mm/dd/yy"
	scheduleHorizon = 3
)

func getCal(args []string) (result string) {
	var start, end *time.Time

	if args == nil { 
		start = time.LocalTime()

		end = time.LocalTime()
		end.Month += scheduleHorizon
	} else if len(args) == 1 {
		start = parseTime(args[0])

		if start == nil {
			return fmt.Sprintf("Couldn't parse '%s' as a date, expected format %s", args[0], timeFormatHelp)
		}

		end = new(time.Time)
		*end = *start
		end.Hour = 23
		end.Minute = 59
	} else {
		start = parseTime(args[0])

		if start == nil {
			return fmt.Sprintf("Couldn't parse '%s' as a date, expected format %s", args[0], timeFormatHelp)
		}

		end = parseTime(args[1])

		if end == nil {
			return fmt.Sprintf("Couldn't parse '%s' as a date, expected format %s", args[1], timeFormatHelp)
		}

		end.Hour = 23
		end.Minute = 59

		if start.Seconds() > end.Seconds() {
			result = "StartDate > EndDate, swapping.\n"
			start, end = end, start
		}
		
	}


	response, _, err := http.Get(fmt.Sprintf("http://www.google.com/calendar/feeds/psuacm%%40cs.pdx.edu/public/basic?start-min=%s&start-max=%s&prettyprint=true&fields=openSearch:totalResults,entry(title,content)", 
		start.Format(time.RFC3339), end.Format(time.RFC3339)))

	if err != nil {
		log.Stderrf("[E] Error retrieving calendar: %v\n", err)
		return "Error retrieving calendar from teh google"
	}
	
	defer response.Body.Close()

	f := &calFeed{}
	err = xml.Unmarshal(response.Body, f)

	if err != nil {
		log.Stderrf("[E] Error parsing calendar xml: %v\n", err)
		return "XML Fail:  Go yell at cbeck for not using json."
	}
	
	for _, event := range f.Entry {
		result = fmt.Sprintf("%s: %s\n", event.Content[6:strings.Index(event.Content, " to")], event.Title) + result
	}
	
	if f.TotalResults == 0 {
		if start.Day == end.Day && start.Month == end.Month && start.Year == end.Year{
			result += fmt.Sprintf("Nothing scheduled on %s", start.Format(timeFormat))
		} else {
			result += fmt.Sprintf("Nothing scheduled in range %s - %s", start.Format(timeFormat), end.Format(timeFormat))
		}
	}

	return
}

func parseTime(raw string) *time.Time {
	var t *time.Time
	var err os.Error

	if raw == "now" || raw == "today" { 
		return time.LocalTime()
	} else if t, err = time.Parse(timeFormat, raw); err == nil {
		if t.Year == 0 {
			t.Year = 2010
		}
		return t
	}	

	return nil
}
