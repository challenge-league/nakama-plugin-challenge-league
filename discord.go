package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/micro/go-micro/v2/logger"

	"github.com/bwmarrin/discordgo"
	nakamaCommands "github.com/challenge-league/nakama-go/commands"
)

const (
	DISCORD_BLOCK_CODE_TYPE = "yaml"
)

type discordSession struct {
	session *discordgo.Session
}

var (
	discordSessionBuilder *discordSession
	discordOnce           sync.Once
)

func NewDiscordSessionSingleton() *discordSession {
	discordOnce.Do(func() {
		s, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
		if err != nil {
			log.Fatalf("Failed to connect to Discord, got %v", err)
		}
		discordSessionBuilder = &discordSession{
			session: s,
		}
	})
	return discordSessionBuilder
}

func (b *discordSession) SetSession(s *discordgo.Session) *discordSession {
	b.session = s
	return b
}

func (b *discordSession) GetSession() *discordgo.Session {
	return b.session
}

func printDiscordChannel(channel *discordgo.Channel, invite *discordgo.Invite) string {
	return "```" + DISCORD_BLOCK_CODE_TYPE + "\n" +
		nakamaCommands.ExecuteTemplate(
			`Channel: {{.Name}}`+"\n",
			channel) +
		nakamaCommands.ExecuteTemplate(`Code: https://discord.gg/{{.Code}}
CreatedAt: {{.CreatedAt}}`,
			invite) + "```"
}

func createDiscordChannels(s *nakamaCommands.MatchState) error {
	channel, _, err := createDiscordChannelWithInvite(
		nakamaCommands.GetUsersFromMatch(s),
		&nakamaCommands.DiscordChannelCreateRequest{
			Name:        s.MatchID,
			ChannelType: discordgo.ChannelTypeGuildText,
			Topic:       s.MatchProfile,
		})
	if err != nil {
		log.Error(err)
		return err
	}
	addDiscordChannelToMatchState(channel, s, false, -1)
	if nakamaCommands.GetMaxUserCountPerTeam(s) == 1 {
		channel, _, err := createDiscordChannelWithInvite(
			nakamaCommands.GetUsersFromMatch(s),
			&nakamaCommands.DiscordChannelCreateRequest{
				Name:        s.MatchID,
				ChannelType: discordgo.ChannelTypeGuildVoice,
				Topic:       s.MatchProfile,
			})
		if err != nil {
			log.Error(err)
			return err
		}
		addDiscordChannelToMatchState(channel, s, false, -1)

	}
	for _, team := range s.Teams {
		if len(team.TeamUsers) > 1 {
			channel, _, err := createDiscordChannelWithInvite(
				nakamaCommands.GetUsersFromTeam(team),
				&nakamaCommands.DiscordChannelCreateRequest{
					Name:        s.MatchID,
					ChannelType: discordgo.ChannelTypeGuildVoice,
					Topic:       s.MatchProfile,
				})
			if err != nil {
				log.Error(err)
				return err
			}
			addDiscordChannelToMatchState(channel, s, false, -1)

			channel, _, err = createDiscordChannelWithInvite(
				nakamaCommands.GetUsersFromTeam(team),
				&nakamaCommands.DiscordChannelCreateRequest{
					Name:        s.MatchID,
					ChannelType: discordgo.ChannelTypeGuildText,
					Topic:       s.MatchProfile,
				})
			if err != nil {
				log.Error(err)
				return err
			}
			addDiscordChannelToMatchState(channel, s, false, -1)
		}
	}

	return nil
}

func addDiscordChannelToMatchState(channel *discordgo.Channel, matchState *nakamaCommands.MatchState, isTeamChannel bool, teamNumber int) {
	if isTeamChannel {
		matchState.Teams[teamNumber].DiscordChannels = append(matchState.Teams[teamNumber].DiscordChannels, &nakamaCommands.DiscordChannel{
			ChannelID:   channel.ID,
			ChannelType: channel.Type,
			GuildID:     channel.GuildID,
		})
	} else {
		matchState.DiscordChannels = append(matchState.DiscordChannels, &nakamaCommands.DiscordChannel{
			ChannelID:   channel.ID,
			ChannelType: channel.Type,
			GuildID:     channel.GuildID,
		})
	}
}

func deleteDiscordChannelsFromMatchState(matchState *nakamaCommands.MatchState) error {
	if err := deleteDiscordChannels(matchState.DiscordChannels); err != nil {
		log.Error(err)
		return err
	}

	for _, team := range matchState.Teams {
		if err := deleteDiscordChannels(team.DiscordChannels); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

func deleteDiscordNewMatchMessageFromMatchState(matchState *nakamaCommands.MatchState) error {
	if err := NewDiscordSessionSingleton().GetSession().ChannelMessageDelete(matchState.DiscordNewMatchMessage.ChannelID, matchState.DiscordNewMatchMessage.ID); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func deleteDiscordChannels(discordChannels []*nakamaCommands.DiscordChannel) error {
	for _, channel := range discordChannels {
		if err := deleteDiscordChannel(channel.ChannelID); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

func deleteDiscordChannel(channelID string) error {
	if channelID != "" {
		_, err := NewDiscordSessionSingleton().GetSession().Channel(channelID)
		if err != nil {
			log.Errorf("Channel %v not found, delete operation canceled", channelID)
			return nil
		}

		if _, err := NewDiscordSessionSingleton().GetSession().ChannelDelete(channelID); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

func createDiscordInviteToChannel(channelID string) (invite *discordgo.Invite, err error) {
	invite, err = NewDiscordSessionSingleton().GetSession().ChannelInviteCreate(channelID, discordgo.Invite{})
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return invite, nil
}

func createDiscordChannel(users []*nakamaCommands.User, discordChannelCreateRequest *nakamaCommands.DiscordChannelCreateRequest) (channel *discordgo.Channel, err error) {
	guildID := os.Getenv("DISCORD_GUILD_ID") // Data League default guildID
	if len(users) > 0 {
		if users[0].Discord.GuildID != "" {
			guildID = users[0].Discord.GuildID
		}
	}
	var permissions []*discordgo.PermissionOverwrite

	/*

		permissions = append(permissions, &discordgo.PermissionOverwrite{
			ID:   guildID, // @everyone in GuildID
			Type: "role",
			Deny: discordgo.PermissionViewChannel,
		})
			roles, err := NewDiscordSessionSingleton().GetSession().GuildRoles(guildID)
			if err != nil {
				log.Error(err)
				return nil, err
			}
			for _, role := range roles {
				if role.Name == "bot" {
					permissions = append(permissions, &discordgo.PermissionOverwrite{
						ID:    role.ID,
						Type:  "role",
						Allow: discordgo.PermissionManageChannels,
					})

					permissions = append(permissions, &discordgo.PermissionOverwrite{
						ID:    role.ID,
						Type:  "role",
						Allow: discordgo.PermissionAdministrator,
					})
				}
			}
	*/
	for _, user := range users {
		// avoid setting permissions for testuser#0-9
		if !strings.Contains(user.Nakama.CustomID, "#") {
			permissions = append(permissions, &discordgo.PermissionOverwrite{
				ID:    user.Nakama.CustomID,
				Type:  "member",
				Allow: discordgo.PermissionViewChannel,
			})
		}
	}

	channel, err = NewDiscordSessionSingleton().GetSession().GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:                 discordChannelCreateRequest.Name,
		Type:                 discordChannelCreateRequest.ChannelType,
		Topic:                discordChannelCreateRequest.Topic,
		PermissionOverwrites: permissions,
	})

	if err != nil {
		log.Error(err)
		return nil, err
	}

	return channel, nil
}

func createDiscordChannelWithInvite(users []*nakamaCommands.User, discordChannelCreateRequest *nakamaCommands.DiscordChannelCreateRequest) (channel *discordgo.Channel, invite *discordgo.Invite, err error) {
	if channel, err = createDiscordChannel(users, discordChannelCreateRequest); err != nil {
		log.Error(err)
		return nil, nil, err
	}

	if invite, err = createDiscordInviteToChannel(channel.ID); err != nil {
		log.Error(err)
		return nil, nil, err
	}

	if channel.Type != discordgo.ChannelTypeGuildText {
		if err := notifyDiscordUsers(users, fmt.Sprint(printDiscordChannel(channel, invite))); err != nil {
			log.Error(err)
			return nil, nil, err
		}
	}

	return channel, invite, nil
}

func notifyDiscordChannel(channelID string, message string) (*discordgo.Message, error) {
	if channelID == "" {
		log.Info("channelID is empty, skipping notification")
		return nil, nil
	}
	msg, err := NewDiscordSessionSingleton().GetSession().ChannelMessageSend(channelID, message)
	if err != nil {
		log.Error(err)
		return msg, err
	}
	return msg, nil
}

func notifyDiscordUser(user *nakamaCommands.User, message string) (*discordgo.Message, error) {
	log.Info(fmt.Sprintf("%+v", user.Discord))
	log.Info(fmt.Sprintf("%+v", user.Discord.AuthorID))
	log.Info(fmt.Sprintf("%+v", user.Discord.ChannelID))
	log.Info(fmt.Sprintf("%+v", user.Discord.GuildID))
	log.Info(fmt.Sprintf("%+v", user.Discord.Username))
	log.Info(fmt.Sprintf("%+v", user.Discord))
	log.Info(fmt.Sprintf("%+v", user))
	return notifyDiscordChannel(user.Discord.ChannelID, message)
}

func notifyDiscordUsers(users []*nakamaCommands.User, message string) error {
	for _, user := range users {
		if user.Discord != nil {
			if _, err := notifyDiscordUser(user, message); err != nil {
				log.Error(err)
				return err
			}
		}
	}
	return nil
}

func createDiscordEmbed(description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Color:       0x00ff00,
		Timestamp:   string(time.Now().UTC().Unix()),
		Description: description,
	}
}

func notifyDiscordEmbed(match *nakamaCommands.MatchState, embed *discordgo.MessageEmbed) error {
	for _, team := range match.Teams {
		for _, teamUser := range team.TeamUsers {
			if teamUser.User.Discord != nil {
				_, err := NewDiscordSessionSingleton().GetSession().ChannelMessageSendEmbed(teamUser.User.Discord.ChannelID, embed)
				if err != nil {
					log.Error(err)
					return err
				}
			}
		}
	}

	return nil
}

func notifyDiscordNewMatch(s *nakamaCommands.MatchState) error {
	message := nakamaCommands.PrintNewMatchMessage(s)
	if err := notifyDiscordUsers(
		nakamaCommands.GetUsersFromMatch(s),
		message,
	); err != nil {
		log.Errorf("Error %+v", err)
		return fmt.Errorf("Failed to notify users for match %v, got %w", s.MatchID, err)
	}
	return nil
}

func notifyDiscordNewMatchEmbed(s *nakamaCommands.MatchState) error {
	embed := createDiscordEmbed(nakamaCommands.PrintNewMatchMessage(s))
	if err := notifyDiscordEmbed(
		s,
		embed,
	); err != nil {
		log.Errorf("Error %+v", err)
		return fmt.Errorf("Failed to notify users for match %v, got %w", s.MatchID, err)
	}
	return nil
}

func init() {
	NewDiscordSessionSingleton()
}
