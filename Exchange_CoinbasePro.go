package main

import (
	"database/sql"
	"encoding/hex"
	. "fmt"
	"github.com/preichenberger/go-coinbasepro/v2"
	cb "github.com/preichenberger/go-coinbasepro/v2"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Coinbase_Auth struct {
	passphrase string
	secretKey  string
	apiToken   string
}

var auth Coinbase_Auth
var coinbaseConfig = setupConfig()
var feePerc = decimal.NewFromFloat(.005)

func setupConfig() *viper.Viper {
	coinbaseConfig := viper.New()
	coinbaseConfig.SetConfigName("coinbase_pro")
	coinbaseConfig.SetConfigType("json")
	coinbaseConfig.AddConfigPath(BaseDir + "/encryption/")
	coinbaseConfig.SetConfigFile(BaseDir + "/encryption/coinbase_pro.json")
	return coinbaseConfig
}

func setupCoinbaseToken() {
	passphrase := readPassword("Enter the passphrase: ")
	secretKey := readPassword("Enter the key: ")
	apiToken := readPassword("Enter the api token: ")
	Println("Encrypting ....")
	encryptPassphrase, _ := Encrypt([]byte(encryptionPass), []byte(passphrase))
	encryptKey, _ := Encrypt([]byte(encryptionPass), []byte(secretKey))
	encryptToken, _ := Encrypt([]byte(encryptionPass), []byte(apiToken))
	coinbaseConfig.SetDefault("passphrase", hex.EncodeToString(encryptPassphrase))
	coinbaseConfig.SetDefault("key", hex.EncodeToString(encryptKey))
	coinbaseConfig.SetDefault("token", hex.EncodeToString(encryptToken))
	err := coinbaseConfig.WriteConfig()
	if err != nil {
		Println(err.Error())
	}
	loadCoinbaseConfig()
}

func loadCoinbaseConfig() {
	if auth == (Coinbase_Auth{}) {
		err := coinbaseConfig.ReadInConfig()
		if err != nil {
			panic(Errorf("Fatal error config file: %s \n", err))
		}
		Println("Decrypting ....")
		decPassphrase, _ := hex.DecodeString(coinbaseConfig.GetString("passphrase"))
		passphrase, _ := Decrypt([]byte(encryptionPass), decPassphrase)
		decKey, _ := hex.DecodeString(coinbaseConfig.GetString("key"))
		key, _ := Decrypt([]byte(encryptionPass), decKey)
		decToken, _ := hex.DecodeString(coinbaseConfig.GetString("token"))
		token, _ := Decrypt([]byte(encryptionPass), decToken)
		auth = Coinbase_Auth{
			passphrase: string(passphrase),
			secretKey:  string(key),
			apiToken:   string(token),
		}
	}
}

func connectToCoinbase() *coinbasepro.Client {
	loadCoinbaseConfig()
	var coinbase = coinbasepro.NewClient()
	coinbase.HTTPClient = &http.Client{
		Timeout: 15 * time.Second,
	}
	coinbase.UpdateConfig(&coinbasepro.ClientConfig{
		BaseURL:    "https://api.pro.coinbase.com",
		Key:        auth.apiToken,
		Passphrase: auth.passphrase,
		Secret:     auth.secretKey,
	})
	return coinbase
}

func startCoinbaseBot(command chan string, settings BotSettings) {
	coinbase := connectToCoinbase()
	updateMarketHistory(coinbase, settings)
	Println("Bot Initalization Complete")
	go run(settings)
}

func getMidMarket(market string, coinbase *coinbasepro.Client) decimal.Decimal {
	ticker, _ := coinbase.GetTicker(market)
	bidPrice, _ := decimal.NewFromString(ticker.Bid)
	askPrice, _ := decimal.NewFromString(ticker.Ask)
	return decimal.Avg(bidPrice, askPrice)
}

func getActiveOrders(coinbase *coinbasepro.Client) []coinbasepro.Order {
	var orders []coinbasepro.Order
	cursor := coinbase.ListOrders()
	for cursor.HasMore {
		if err := cursor.NextPage(&orders); err != nil {
			return nil
		}
	}
	if len(orders) == 0 {
		if err := cursor.PrevPage(&orders); err != nil {
			return nil
		}
	}
	return orders
}

func getFills(coinbase *coinbasepro.Client, market string) []coinbasepro.Fill {
	var fills []coinbasepro.Fill
	cursor := coinbase.ListFills(coinbasepro.ListFillsParams{
		OrderID:    "",
		ProductID:  market,
		Pagination: coinbasepro.PaginationParams{},
	})
	for cursor.HasMore {
		if err := cursor.NextPage(&fills); err != nil {
			return nil
		}
	}
	if len(fills) == 0 {
		if err := cursor.PrevPage(&fills); err != nil {
			return nil
		}
	}
	return fills
}

func getLastPurchase(coinbase *coinbasepro.Client, market string, t string) coinbasepro.Fill {
	fills := getFills(coinbase, market)
	for _, fill := range fills {
		if fill.Side == t {
			return fill
		}
	}
	return coinbasepro.Fill{}
}

func PlaceOrder(coinbase *coinbasepro.Client, t string, market string, amount decimal.Decimal, price decimal.Decimal) {
	for _, o := range getActiveOrders(coinbase) { // Check for current orders matching this one
		if o.ProductID == market {
			if strings.EqualFold(t, o.Side) {
				orderPrice, _ := decimal.NewFromString(o.Price)
				if !(orderPrice.Equals(price)) {
					err := coinbase.CancelOrder(o.ID)
					if err != nil {
						Println("Failed to cancel order! (" + o.ID + ")(" + o.Size + " @ " + o.Price + ")")
						return
					}
					Println("Canceling order (" + o.Size + " @ " + o.Price + ")")
				} else {
					Println("Keeping Order (" + o.Size + " @ " + o.Price + ")")
					return
				}
			}
		}
	}
	order := coinbasepro.Order{
		Price:     price.String(),
		Size:      amount.String(),
		Side:      t,
		ProductID: market,
	}
	_, err := coinbase.CreateOrder(&order)
	if err != nil {
		Println("Failed to place order!")
		Println(err)
		return
	} else {
		Println("Placed " + t + "Order for " + market + " for " + amount.String() + " @ $" + price.String())
	}
}

func GetMarketDecimal(coinbase *coinbasepro.Client, market string) [2]int {
	products, _ := coinbase.GetProducts()
	for _, product := range products {
		if strings.EqualFold(product.ID, market) {
			return [2]int{strings.Index(product.QuoteIncrement, "1"), strings.Index(product.BaseMinSize, "1")}
		}
	}
	return [2]int{0, 0}
}

func GetTotalMoney(coinbase *coinbasepro.Client, currencyType string) decimal.Decimal {
	accounts, err := coinbase.GetAccounts()
	if err != nil {
		Println("Failed to connect, Invalid Token's")
	} else {
		for _, a := range accounts {
			bal, err := decimal.NewFromString(a.Balance)
			if err != nil {
				panic(err)
			}
			if bal.GreaterThan(decimal.NewFromInt(0)) {
				if a.ID == currencyType {
					return bal
				}
			}
		}
	}
	return decimal.NewFromFloat(0)
}

type MarketData struct {
	market string
	buys   map[string]coinbasepro.Message
	sells  map[string]coinbasepro.Message
}

func updateMarketHistory(coinbase *coinbasepro.Client, settings BotSettings) {
	Println("Updating Market History")
	sql := ConnectDB()
	startTimestmap := int64(1420088400) // Jan 1, 2015
	// Get Latest timestamp and update from there
	query, err := sql.Query("SELECT MAX(timestamp) FROM market_data WHERE market='" + settings.Market + "'")
	if err != nil {
		println(err.Error())
	}
	if query != nil && query.Next() {
		err = query.Scan(&startTimestmap)
		if err != nil {
			println(err.Error())
		}
	}
	timeRemaining := time.Now().Unix() - startTimestmap
	Println("Currently " + strconv.Itoa(int(timeRemaining)/60) + " entries missing!")
	updateMarketData(settings.Market, coinbase, startTimestmap, sql)
}

func updateMarketData(market string, coinbase *coinbasepro.Client, timestamp int64, sql *sql.DB) {
	increment := int64(300 * 60) // 300 entires in 1m increments
	for {
		rates := cb.GetHistoricRatesParams{
			Start:       time.Unix(timestamp, 0),
			End:         time.Unix(timestamp+increment, 0),
			Granularity: 60,
		}
		history, err := coinbase.GetHistoricRates(market, rates)
		if err != nil {
			println(err.Error())
		}
		for _, timeHistory := range history {
			_, err := sql.Exec("INSERT INTO market_data (exchange, market, timestamp, lowest_price, highest_price, first_trade_price, last_trade_price, volume) VALUES " +
				"('" + "coinbase_pro" + "', '" + market + "', '" + strconv.Itoa(int(timeHistory.Time.Unix())) + "', '" + strconv.FormatFloat(timeHistory.Low, 'g', 8, 64) + "', '" + strconv.FormatFloat(timeHistory.High, 'g', 8, 64) +
				"', '" + strconv.FormatFloat(timeHistory.Open, 'g', 8, 64) + "', '" + strconv.FormatFloat(timeHistory.Close, 'g', 8, 64) + "', '" + strconv.FormatFloat(timeHistory.Volume, 'g', 8, 64) + "')")
			if err != nil {
				println(err.Error())
			}
		}
		if len(history) > 0 {
			Println("Added " + strconv.Itoa(len(history)) + " Entries to DB, Currently at " + history[0].Time.Format("2006-01-02 15:04:05"))
		} else {
			Println("Looking for when the history starts " + time.Unix(timestamp, 0).Format("2006-01-02 15:04:05"))
		}
		if timestamp >= time.Now().Unix() {
			break
		}
		timestamp = timestamp + increment
		time.Sleep(350 * time.Millisecond) // 1.08s for 3, below 1 per sec, as per ratelimit
	}
	Println("Historical Data Updated")
}
