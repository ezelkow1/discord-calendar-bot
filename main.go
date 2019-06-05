package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

//Configuration for bot
type Configuration struct {
	Token            string
	BroadcastChannel string
	DbFile           string
}

// Event for calendar
type Event struct {
	Name     string
	Date     time.Time
	notifies []string
}

// Variables used for command line parameters or global
var (
	config      = Configuration{}
	configfile  string
	embedColor  = 0x00ff00
	initialized = false
	guildID     string
	roleID      string
	fileLock    sync.RWMutex
	x           []Event
	layout      = "1/2/2006 15:04"
	output      = "Mon Jan 2 15:04"
)

func init() {
	flag.StringVar(&configfile, "c", "", "Configuration file location")
	flag.Parse()

	if configfile == "" {
		fmt.Println("No config file entered")
		os.Exit(1)
	}

	if _, err := os.Stat(configfile); os.IsNotExist(err) {
		fmt.Println("Configfile does not exist, you should make one")
		os.Exit(2)
	}

	fileh, _ := os.Open(configfile)
	decoder := json.NewDecoder(fileh)
	err := decoder.Decode(&config)
	if err != nil {
		fmt.Println("error: ", err)
		os.Exit(3)
	}
}

func main() {

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register ready as a callback for the ready events.
	dg.AddHandler(ready)

	// Register messageCreate as a callback for message events
	dg.AddHandler(messageCreate)

	if _, err := os.Stat(config.DbFile); os.IsNotExist(err) {
		fmt.Println("Db File does not exist, creating")
		newFile, _ := os.Create(config.DbFile)
		newFile.Close()
	}

	fileLock.Lock()
	Load(config.DbFile, &x)
	checkEvents()
	fileLock.Unlock()
	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

func createTimer(thisevent Event, s *discordgo.Session) {
	go func() {
		timer := time.NewTimer(thisevent.Date.Sub(time.Now()))
		<-timer.C

		SendEmbed(s, config.BroadcastChannel, "", "Event Starting", "Event for "+thisevent.Name+" is starting now")
		Load(config.DbFile, &x)
		for index := range x {
			if thisevent.Name == x[index].Name {
				thisevent = x[index]
			}
		}

		for _, mentions := range thisevent.notifies {
			if mentions != "" {
				s.ChannelMessageSend(config.BroadcastChannel, mentions)
			}
		}
		timer.Stop()
		deleteOneEvent(thisevent.Name)
	}()
}

func initEvents(s *discordgo.Session) {
	for _, thisevent := range x {
		createTimer(thisevent, s)
	}
}

func checkEvents() {
	mytime := time.Now()
	for _, thisevent := range x {
		if thisevent.Date.Before(mytime) {
			// Loops twice, but meh its startup
			deleteOneEvent(thisevent.Name)
		}
	}
	Save(config.DbFile, &x)
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func ready(s *discordgo.Session, event *discordgo.Ready) {

	// Discord just loves to send ready events during server hiccups
	// This prevents spamming
	if initialized == false {
		// Set the playing status.
		s.UpdateStatus(0, "")
		//SendEmbed(s, config.BroadcastChannel, "", "I iz here", "Keybot has arrived. You may now use me like the dumpster I am")
		initialized = true
		guildID = event.Guilds[0].ID
		initEvents(s)
	}
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Only allow messages in either DM or broadcast channel
	dmchan, err := s.UserChannelCreate(m.Author.ID)

	if err != nil {
		fmt.Println("error: ", err)
		fmt.Println("messageCreate err in creating dmchan")
		return
	}

	if (m.ChannelID != config.BroadcastChannel) && (m.ChannelID != dmchan.ID) {
		return
	}

	// Skip any messages we dont care about
	if checkPrefix(m.Content) == false {
		return
	}

	// Add a new key to the db
	if strings.HasPrefix(m.Content, "!add ") == true {
		addEvent(s, m)
	}

	if strings.HasPrefix(m.Content, "!list") == true {
		listEvents(s, m)
	}

	if strings.HasPrefix(m.Content, "!delete ") == true {
		deleteEvent(s, m)
	}

	if strings.HasPrefix(m.Content, "!time") == true {
		printTime(s, m)
	}

	if strings.HasPrefix(m.Content, "!help") == true {
		printHelp(s, m)
	}

	if strings.HasPrefix(m.Content, "!notify") == true {
		addNotify(s, m)
	}
}

func printHelp(s *discordgo.Session, m *discordgo.MessageCreate) {
	var buffer bytes.Buffer
	buffer.WriteString("!add Event Name Date Time (in EDT) - i.e. Bean Battles 05/29/2019 17:00\n")
	buffer.WriteString("!notify Event Name @member @role ...... etc - Adds the members and roles to a list of notifications for the event")
	buffer.WriteString("!list - Lists current events scheduled and their times\n")
	buffer.WriteString("!delete Event Name - Removes an event with the Event Name\n")
	buffer.WriteString("!time - prints the current date and time in EDT\n")
	buffer.WriteString("!help - you're looking at it\n")
	SendEmbed(s, m.ChannelID, "", "Available Commands", buffer.String())
}

func printTime(s *discordgo.Session, m *discordgo.MessageCreate) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		SendEmbed(s, m.ChannelID, "", "TimeZone Error", "Error loading timezone data")
	}
	SendEmbed(s, m.ChannelID, "", "Current Time (EDT)", "The current time is: "+time.Now().In(newYork).Format(output)+" EDT")
}

func addNotify(s *discordgo.Session, m *discordgo.MessageCreate) {
	msg := strings.TrimPrefix(m.Content, "!notify ")
	name := ""
	Load(config.DbFile, &x)
	for index := range x {
		if strings.Contains(msg, x[index].Name) {
			msg = strings.TrimPrefix(msg, x[index].Name)
			name = x[index].Name
			mentions := strings.Split(msg, " ")
			for _, names := range mentions {
				if names != "" {
					x[index].notifies = append(x[index].notifies, names)
				}
			}
		}
	}

	if name != "" {
		Save(config.DbFile, &x)
		SendEmbed(s, m.ChannelID, "", "Added Notifications", "Added the mentions to the list for "+name)
	} else {
		SendEmbed(s, m.ChannelID, "", "No Event with that Name", "No event with that name found")
	}

}

func addEvent(s *discordgo.Session, m *discordgo.MessageCreate) {
	//layout      = "01/02/2006 15:04"
	thisevent := strings.Split(m.Content, " ")

	timeval := strings.Join(thisevent[len(thisevent)-2:], " ")
	eventname := strings.Join(thisevent[1:len(thisevent)-2], " ")
	var this Event
	this.Name = eventname

	if this.Name == "" {
		SendEmbed(s, m.ChannelID, "", "Missing Name", "Your event needs a name")
		return
	}

	var err error
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		SendEmbed(s, m.ChannelID, "", "TimeZone Error", "Error loading timezone data")
		return
	}
	this.Date, err = time.ParseInLocation(layout, timeval, newYork)
	if err != nil {
		SendEmbed(s, m.ChannelID, "", "Format Error", "Check your date format, I dont think its right")
		return
	}

	mytime := time.Now()
	if this.Date.Before(mytime) {
		SendEmbed(s, m.ChannelID, "", "Too early", "Silly, dont make an event before right now")
		return
	}
	this.notifies = append(this.notifies, m.Author.Mention())

	fileLock.Lock()
	Load(config.DbFile, &x)
	x = append(x, this)
	Save(config.DbFile, &x)
	fileLock.Unlock()

	SendEmbed(s, m.ChannelID, "", "Event Created", "Created Event: "+this.Name+" at "+this.Date.Format(time.RFC1123))
	s.ChannelMessageSend(config.BroadcastChannel, this.notifies[0])
	createTimer(this, s)
}

func listEvents(s *discordgo.Session, m *discordgo.MessageCreate) {
	var buffer bytes.Buffer

	Load(config.DbFile, &x)
	if len(x) == 0 {
		SendEmbed(s, m.ChannelID, "", "No Events", "No Events Scheduled")
	} else {
		for _, events := range x {
			buffer.WriteString(events.Name + " at " + events.Date.Format(output) + " EDT\n")
		}
	}
	SendEmbed(s, m.ChannelID, "", "Events", buffer.String())
}

func deleteEvent(s *discordgo.Session, m *discordgo.MessageCreate) {
	m.Content = strings.TrimPrefix(m.Content, "!delete ")
	Load(config.DbFile, &x)

	if deleteOneEvent(m.Content) {
		SendEmbed(s, m.ChannelID, "", "Deleted Event", "Deleted Event: "+m.Content)
	} else {
		SendEmbed(s, m.ChannelID, "", "Event not found", "No Event with the name "+m.Content+" was found")
	}
	return
}

func deleteOneEvent(name string) bool {
	for i, event := range x {
		hasstring := strings.Compare(name, event.Name)
		if hasstring == 0 {
			x = append(x[0:i], x[i+1:]...)
			Save(config.DbFile, &x)
			return true
		}
	}

	return false
}
