package bruxism

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"github.com/iopred/comicgen"
)

type comicPlugin struct {
	SimplePlugin
	log map[string][]Message
}

func (p *comicPlugin) helpFunc(bot *Bot, service Service, detailed bool) []string {
	help := []string{
		commandHelp(service, "comic", "[<1-10>]", "Creates a comic from recent messages.")[0],
		commandHelp(service, "customcomic", "[<id>:] <text> | [<id>:] <text>", "Creates a custom comic.")[0],
		commandHelp(service, "customcomicsimple", "[<id>:] <text> | [<id>:] <text>", "Creates a simple custom comic.")[0],
	}

	if detailed {
		ticks := ""
		if service.Name() == DiscordServiceName {
			ticks = "`"
		}
		help = append(help, []string{
			"Examples:",
			fmt.Sprintf("%s%scomic 5%s - Creates a comic from the last 5 messages.", ticks, service.CommandPrefix(), ticks),
			fmt.Sprintf("%s%scustomcomic Hello | 1: World | Yeah!%s - Creates a comic with 3 lines, with the second line being spoken by a different character.", ticks, service.CommandPrefix(), ticks),
			fmt.Sprintf("%s%scustomcomicsimple Foo | 1: Bar%s - Creates a comic with 2 lines, both spoken by different characters.", ticks, service.CommandPrefix(), ticks),
		}...)
	}

	return help
}

func makeScriptFromMessages(service Service, message Message, messages []Message) *comicgen.Script {
	speakers := make(map[string]int)
	avatars := make(map[int]string)

	script := []*comicgen.Message{}

	for _, message := range messages {
		speaker, ok := speakers[message.UserName()]
		if !ok {
			speaker = len(speakers)
			speakers[message.UserName()] = speaker
			avatars[speaker] = message.UserAvatar()
		}

		script = append(script, &comicgen.Message{
			Speaker: speaker,
			Text:    message.Message(),
			Author:  message.UserName(),
		})
	}
	return &comicgen.Script{
		Messages: script,
		Author:   fmt.Sprintf(service.UserName()),
		Avatars:  avatars,
		Type:     comicgen.ComicTypeChat,
	}
}

func (p *comicPlugin) makeComic(bot *Bot, service Service, message Message, script *comicgen.Script) {
	comic := comicgen.NewComicGen("arial", service.Name() != DiscordServiceName)
	image, err := comic.MakeComic(script)
	if err != nil {
		service.SendMessage(message.Channel(), fmt.Sprintf("Sorry %s, there was an error creating the comic. %s", message.UserName(), err))
	} else {
		go func() {
			url, err := bot.UploadToImgur(image, "comic.png")
			if err == nil {
				if service.Name() == DiscordServiceName {
					service.SendMessage(message.Channel(), fmt.Sprintf("Here's your comic <@%s>: %s", message.UserID(), url))
				} else {
					service.SendMessage(message.Channel(), fmt.Sprintf("Here's your comic %s: %s", message.UserName(), url))
				}
			} else {
				fmt.Println(err)
				service.SendMessage(message.Channel(), fmt.Sprintf("Sorry %s, there was a problem uploading the comic to imgur.", message.UserName()))
			}
		}()
	}
}

func (p *comicPlugin) messageFunc(bot *Bot, service Service, message Message) {
	if service.IsMe(message) {
		return
	}

	log, ok := p.log[message.Channel()]
	if !ok {
		log = []Message{}
	}

	if matchesCommand(service, "customcomic", message) || matchesCommand(service, "customcomicsimple", message) {
		ty := comicgen.ComicTypeChat
		if matchesCommand(service, "customcomicsimple", message) {
			ty = comicgen.ComicTypeSimple
		}

		service.Typing(message.Channel())

		str, _ := parseCommand(service, message)

		messages := []*comicgen.Message{}

		splits := strings.Split(str, "|")
		for _, line := range splits {
			line := strings.Trim(line, " ")

			text := ""
			speaker := 0
			if strings.Index(line, ":") != -1 {
				lineSplit := strings.Split(line, ":")

				speaker, _ = strconv.Atoi(strings.Trim(lineSplit[0], " "))

				text = strings.Trim(lineSplit[1], " ")
			} else {
				text = line
			}

			messages = append(messages, &comicgen.Message{
				Speaker: speaker,
				Text:    text,
			})
		}

		if len(messages) == 0 {
			service.SendMessage(message.Channel(), fmt.Sprintf("Sorry %s, you didn't add any text.", message.UserName()))
			return
		}

		p.makeComic(bot, service, message, &comicgen.Script{
			Messages: messages,
			Author:   fmt.Sprintf(service.UserName()),
			Type:     ty,
		})
	} else if matchesCommand(service, "comic", message) {
		if len(log) == 0 {
			service.SendMessage(message.Channel(), fmt.Sprintf("Sorry %s, I don't have enough messages to make a comic yet.", message.UserName()))
			return
		}

		service.Typing(message.Channel())

		lines := 0
		linesString, parts := parseCommand(service, message)
		if len(parts) > 0 {
			lines, _ = strconv.Atoi(linesString)
		}

		if lines <= 0 {
			lines = 1 + int(math.Floor((math.Pow(2*rand.Float64()-1, 3)/2+0.5)*float64(5)))
		}

		if lines > len(log) {
			lines = len(log)
		}

		p.makeComic(bot, service, message, makeScriptFromMessages(service, message, log[len(log)-lines:]))
	} else {
		// Don't append commands.
		if strings.HasPrefix(strings.ToLower(strings.Trim(message.Message(), " ")), strings.ToLower(service.CommandPrefix())) {
			return
		}

		switch message.Type() {
		case MessageTypeCreate:
			if len(log) < 10 {
				log = append(log, message)
			} else {
				log = append(log[1:], message)
			}
		case MessageTypeUpdate:
			for i, m := range log {
				if m.MessageID() == message.MessageID() {
					log[i] = message
					break
				}
			}
		case MessageTypeDelete:
			for i, m := range log {
				if m.MessageID() == message.MessageID() {
					log = append(log[:i], log[i+1:]...)
					break
				}
			}
		}
		p.log[message.Channel()] = log
	}
}

// NewComicPlugin will create a new top streamers plugin.
func NewComicPlugin() Plugin {
	p := &comicPlugin{
		SimplePlugin: *NewSimplePlugin("Comic"),
		log:          make(map[string][]Message),
	}
	p.message = p.messageFunc
	p.help = p.helpFunc
	return p
}
