package main

import (
	"bufio"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"os"
	"sync"
)

var BaseDir = "./crypto"

var coreConfig viper.Viper

func main() {
	_, err := os.Stat(BaseDir)
	if os.IsNotExist(err) {
		err2 := os.Mkdir(BaseDir, 0755)
		if err2 != nil {
			log.Fatal(err)
		}
	}
	readCoreConfig()
	if coreConfig.GetBool("debug_enabled") {
		fmt.Println("Debug mode enabled")
	}
	setupOrLoadEncryptionKey()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		handleCommands(wg)
	}()
	select {}
}

func handleCommands(wg sync.WaitGroup) {
	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print(": ")
		command, _ := reader.ReadString('\n')
		runCommands(command, wg)
	}
}

func readCoreConfig() {
	coreConfig := viper.New()
	coreConfig.SetConfigName("core")
	coreConfig.SetConfigType("json")
	coreConfig.AddConfigPath(BaseDir)
	// Set Defaults
	coreConfig.SetDefault("debug_enabled", false)
	coreConfig.SafeWriteConfig()
	// Watch for updates
	// Read config
	err := coreConfig.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("Fatal error config file: %s \n", err))
	}
}
