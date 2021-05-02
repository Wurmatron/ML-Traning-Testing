package main

import (
	"fmt"
	"github.com/shopspring/decimal"
	"os"
	"strings"
	"sync"
)

var commands map[string]func([]string)

// Setup the possible command line commands
func addCommands() {
	commands = make(map[string]func([]string))
	commands["connect"] = connect
	commands["exchange"] = exchange
	commands["start"] = startupBot
}

// Remove the provided amount of 's' from the begging of a string array
func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

// Run a command
func runCommands(command string, wg sync.WaitGroup) {
	command = strings.Replace(command, "\n", "", -1)
	args := strings.Split(command, " ")
	name := strings.ToLower(args[0])
	if len(args) > 0 {
		if len(commands) == 0 {
			addCommands()
		}
		if strings.EqualFold(name, "quit") {
			wg.Done()
		}
		if commands[name] != nil {
			commands[name](remove(args, 0))
		} else {
			fmt.Println("Invalid Command! type 'help' for a full list")
		}
	}
}

// Run the prefixed 'connect' command
func connect(args []string) {
	if len(args) == 1 {
		if strings.EqualFold(args[0], "list") {
			fmt.Println("Supported Exchanges: [coinbase_pro]")
		} else if strings.EqualFold(args[0], "coinbase_pro") {
			setupCoinbaseToken()
		} else {
			fmt.Println("Invalid Exchange!")
		}
	} else {
		fmt.Println("connect <exchange>")
	}
}

// Run the prefixed 'exchange' command
func exchange(args []string) {
	if len(args) == 2 {
		if strings.EqualFold(args[0], "coinbase_pro") {
			coinbase := connectToCoinbase()
			if strings.EqualFold(args[1], "balance") {
				accounts, err := coinbase.GetAccounts()
				if err != nil {
					fmt.Println("Failed to connect, Invalid Token's")
				} else {
					fmt.Println("Connected to CoinBase Pro!")
					for _, a := range accounts {
						bal, err := decimal.NewFromString(a.Balance)
						if err != nil {
							panic(err)
						}
						if bal.GreaterThan(decimal.NewFromInt(0)) {
							fmt.Println("You have " + a.Balance + " " + a.Currency)
						}
					}
				}
			}
		} else {
			fmt.Println("Invalid Exchange!")
		}
	} else {
		fmt.Println("exchange <exchange> test")
	}
}

// Startup the bot running on a single provided market
// TODO Run multiple bots based on its name, via start <name>
func startupBot(args []string) {
	var encryptionDir = BaseDir + "/encryption/coinbase_pro.json"
	_, err := os.Stat(encryptionDir)
	if !(os.IsNotExist(err)) {
		ConnectDB()
		commandBot := make(chan string)
		fmt.Println("Starting Bot...")
		go startCoinbaseBot(commandBot, BotSettings{
			Name:                  "Bot",
			Market:                "XLM-USD",
			UpdateTime:            300,
			MarginSell:            0.01,
			MarginBuy:             0.01,
			AmountCalculationType: "SetCurrency",
			AmountData:            "5",
		})
	} else {
		fmt.Println("You must first connect to coinbase pro!")
		fmt.Println("connect coinbase_pro")
	}
}
