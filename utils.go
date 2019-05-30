package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

//CleanKey cleans up the input name. Strips trailing key from input
func CleanKey(name string, key string) string {
	tmp := strings.TrimSuffix(name, key)
	tmp = strings.TrimSpace(tmp)
	return tmp
}

//NormalizeGame the name of the game, removes spaces, lowercases
func NormalizeGame(name string) string {
	tmp := strings.ToLower(name)
	tmp = strings.Replace(tmp, " ", "", -1)
	return tmp
}

// Save via json to file
func Save(path string, object interface{}) {
	b, err := json.Marshal(object)
	if err != nil {
		fmt.Println("error on marshall")
	}
	fileh, err := os.Create(path)
	n, err := fileh.Write(b)
	b = b[:n]
	fileh.Close()
	return
}

// Load json file
func Load(path string, object interface{}) {
	fileh, err := os.Open(path)
	fileinfo, err := fileh.Stat()
	_ = err
	b := make([]byte, fileinfo.Size())
	n, err := fileh.Read(b)
	if err != nil {
		fmt.Println(err)
		return
	}
	b = b[:n]
	json.Unmarshal(b, &object)
	fileh.Close()
	return
}

// Check if a msg has a prefix we care about. This is for
// optimization so we can skip any messages we dont care about.
// If adding new message triggers they must be added here
func checkPrefix(msg string) bool {

	if (msg == "!listkeys") ||
		(strings.HasPrefix(msg, "!add ") == true) ||
		(strings.HasPrefix(msg, "!list") == true) ||
		(strings.HasPrefix(msg, "!delete ") == true) ||
		(strings.HasPrefix(msg, "!time") == true) ||
		(strings.HasPrefix(msg, "!help") == true) {
		return true
	}

	return false
}

// This function returns roleID from name
func findRolesID(s *discordgo.Session, myrole string) string {

	roles, err := s.GuildRoles(guildID)

	if err == nil {
		for roleids := range roles {
			if NormalizeString(roles[roleids].Name) == NormalizeString(myrole) {
				return roles[roleids].ID
			}
		}
	}

	return ""
}

// This function returns role name from ID
func findRolesName(s *discordgo.Session, myID string) string {
	roles, err := s.GuildRoles(guildID)

	if err == nil {
		for roleids := range roles {
			if roles[roleids].ID == myID {
				return roles[roleids].Name
			}
		}
	}

	return ""
}

// Returns mention string for given name of role
func findRolesMention(s *discordgo.Session, myRole string) string {
	roles, err := s.GuildRoles(guildID)

	if err == nil {
		for roleids := range roles {
			if NormalizeString(roles[roleids].Name) == NormalizeString(myRole) {
				return roles[roleids].Mention()
			}
		}
	}

	return ""
}

// NormalizeString lowercases and strips spaces
func NormalizeString(name string) string {
	tmp := strings.ToLower(name)
	tmp = strings.Replace(tmp, " ", "", -1)
	return tmp
}
