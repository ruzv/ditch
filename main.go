package main

import (
	"context"
	"ditch/conversation"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

func main() {
	logging.SetBackend(
		logging.NewBackendFormatter(
			logging.NewLogBackend(os.Stdout, "", 0),
			logging.MustStringFormatter(
				`%{color}%{time:15:04:05.0000} `+
					`%{level:.4s} %{id:03x} %{module}.%{longfunc} `+
					`▶%{color:reset} %{message}`,
			),
		),
	)

	log, err := logging.GetLogger("ditch")
	if err != nil {
		panic(err)
	}

	log.Info("Starting Ditch")

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.SetEnvPrefix("DITCH")
	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil && !errors.As(err, &viper.ConfigFileNotFoundError{}) {
		panic(err)
	}

	sess, err := discordgo.New(fmt.Sprintf("Bot %s", viper.GetString("discord_token")))
	if err != nil {
		panic(err)
	}

	sess.Identify.Intents = discordgo.IntentsGuildMessages

	cm := conversation.NewConversationManager(
		viper.GetString("openai_secret_key"),
		log,
	)

	sess.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// Ignore all messages created by the bot itself
		// This isn't required in this specific example but it's a good practice.
		if m.Author.ID == s.State.User.ID {
			return
		}

		reply, err := cm.GetConversation(
			conversation.ConversationID{
				UserID:    m.Author.ID,
				ChannelID: m.ChannelID,
			},
		).
			Banter(context.Background(), m.Content)
		if err != nil {
			log.Errorf("Banter error: %v", err)

			return
		}

		err = sendReply(s, m.ChannelID, reply)
		if err != nil {
			log.Errorf("Failed to send reply: %v", err)

			return
		}
	})

	err = sess.Open()
	if err != nil {
		panic(err)
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	err = sess.Close()
	if err != nil {
		panic(err)
	}
}

func sendReply(s *discordgo.Session, channelID string, message string) error {
	for len(message) > 0 {
		msgLen := len(message)

		if msgLen > 2000 {
			msgLen = 2000
		}

		_, err := s.ChannelMessageSend(channelID, message[:msgLen])
		if err != nil {
			return errors.Wrap(err, "failed to send message to channel")
		}

		message = message[msgLen:]
	}

	return nil
}
