package main

import (
	"fmt"
	"github.com/shopspring/decimal"
	"os"
	"strings"
	"sync"
)

var commands map[string]func([]string)

func addCommands() {
	commands = make(map[string]func([]string))
	commands["connect"] = connect
	commands["exchange"] = exchange
	commands["start"] = startupBot
}

func remove(slice []string, s int) []string {
	return append(slice[:s], slice[s+1:]...)
}

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

func exchange(args []string) {
	if len(args) == 2 {
		if strings.EqualFold(args[0], "coinbase_pro") {
			coinbase := connectToCoinbase()
			if strings.EqualFold(args[0], "balance") {
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

func startupBot(args []string) {
	if len(args) == 1 {
		if strings.EqualFold(args[0], "coinbase_pro") {
			var encryptionDir = BaseDir + "/encryption/coinbase_pro.json"
			_, err := os.Stat(encryptionDir)
			if !(os.IsNotExist(err)) {
				commandBot := make(chan string)
				go startCoinbaseBot("LTC-USD", commandBot)
			} else {
				fmt.Println("You must first connect to coinbase pro!")
				fmt.Println("connect coinbase_pro")
			}
		} else {
			fmt.Println("Invalid Exchange!")
		}
	} else {
		fmt.Println("start <exchange>")
	}
}
