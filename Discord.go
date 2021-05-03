package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
)

var discordConfig viper.Viper

func readDiscordConfig() viper.Viper {
	dbConfig := viper.New()
	dbConfig.SetConfigName("discord")
	dbConfig.SetConfigType("json")
	dbConfig.AddConfigPath(BaseDir)
	// Set Defaults
	dbConfig.SetDefault("token", "")
	dbConfig.SetDefault("logChannel", "")
	// Watch for updates
	// Read config
	if err := dbConfig.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			dbConfig.SafeWriteConfig()
		} else {
			panic(err)
		}
	}
	return *dbConfig
}

func StartupDiscordBot() *discordgo.Session {
	discordConfig = readDiscordConfig()
	discord, err := discordgo.New("Bot " + discordConfig.GetString("token"))
	if err != nil {
		panic("Failed to login to discord, Invalid Token")
	}
	if len(discordConfig.GetString("logChannel")) == 0 {
		fmt.Println("Log Chanel Must be configured")
	}
	go func() {
		err = discord.Open()
		if err != nil {
			fmt.Println("error opening connection,", err)
		}
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
		<-sc
	}()
	return discord
}

func BotLog(discord *discordgo.Session, msg string) {
	_, err := discord.ChannelMessageSend(discordConfig.GetString("logChannel"), msg)
	if err != nil {
		println("Invalid Discord Log Channel, Message Failed to be sent!")
		println(err.Error())
	}
}
