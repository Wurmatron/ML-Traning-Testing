package main

import (
	"encoding/hex"
	. "fmt"
	ws "github.com/gorilla/websocket"
	"github.com/preichenberger/go-coinbasepro/v2"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"
	"net/http"
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
var margin = decimal.NewFromFloat(.01)
var purchaseAmount = decimal.NewFromFloat(.1)

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

func startCoinbaseWSS(market string, ch chan MarketData, ask chan int, command chan string) {
	var wsDialer ws.Dialer
	wsConn, _, err := wsDialer.Dial("wss://ws-feed.pro.coinbase.com", nil)
	if err != nil {
		Println(err.Error())
	}
	subscribe := coinbasepro.Message{
		Type: "subscribe",
		Channels: []coinbasepro.MessageChannel{
			coinbasepro.MessageChannel{
				Name: "heartbeat",
				ProductIds: []string{
					market,
				},
			},
			coinbasepro.MessageChannel{
				Name: "full",
				ProductIds: []string{
					market,
				},
			},
		},
	}
	if err := wsConn.WriteJSON(subscribe); err != nil {
		Println(err.Error())
	}
	buys := make(map[string]coinbasepro.Message)
	sells := make(map[string]coinbasepro.Message)
	for true {
		select {
		case <-ask: // Send updated data
			ch <- MarketData{
				market: market,
				buys:   buys,
				sells:  sells,
			}
		case command := <-command:
			if strings.EqualFold(command, "stop") {
				return
			}
		default:
			message := coinbasepro.Message{}
			if err := wsConn.ReadJSON(&message); err != nil {
				Println(err.Error())
				break
			}
			if message.Type == "open" {
				updateOrderBook(message, buys, sells)
			} else if message.Type == "done" {
				updateOrderBook(message, buys, sells)
			}
		}
	}
}

func updateOrderBook(message coinbasepro.Message, buys map[string]coinbasepro.Message, sells map[string]coinbasepro.Message) {
	if message.Type == "open" {
		if message.Side == "buy" {
			buys[message.OrderID] = message
		} else if message.Side == "sell" {
			sells[message.OrderID] = message
		}
	} else if message.Type == "done" {
		if message.Side == "buy" {
			delete(buys, message.OrderID)
		} else if message.Side == "sell" {
			delete(sells, message.OrderID)
		}
	}
}

func startCoinbaseBot(market string, command chan string) {
	coinbase := connectToCoinbase()
	ch := make(chan MarketData)
	askCh := make(chan int)
	go startCoinbaseWSS(market, ch, askCh, command)
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				askCh <- 1
				data := <-ch
				updateBid(coinbase, data)
			case command := <-command:
				if strings.EqualFold(command, "stop") {
					ticker.Stop()
					close(ch)
					close(askCh)
					return
				}
			}
		}
	}()
}

func updateBid(coinbase *coinbasepro.Client, data MarketData) {
	var market = data.market
	var coinName = strings.Split(market, "-")[0]
	amountOnCurrentMarket, _ := decimal.NewFromString(getCoinAmount(coinbase, coinName))
	midMarket := getMidMarket(market, coinbase)
	lastPurchase := getLastPurchase(coinbase, market, "buy")
	buyPrice := getBuyPrice(purchaseAmount, midMarket)
	lastPrice, _ := decimal.NewFromString(lastPurchase.Price)
	sellPrice := getSellPrice(purchaseAmount, midMarket, lastPrice).Round(2) // TODO Dynamic Round according to coin
	if amountOnCurrentMarket.GreaterThan(decimal.NewFromInt(0)) {            // Sell current coins
		placeOrder(coinbase, "sell", market, amountOnCurrentMarket, sellPrice)
		Println("Selling " + amountOnCurrentMarket.String() + coinName + " for $" + sellPrice.String())
	} else { // Buy Coins
		placeOrder(coinbase, "buy", market, purchaseAmount, sellPrice)
		Println("Buying " + purchaseAmount.String() + coinName + " for $" + buyPrice.String())
	}
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

func getSellPrice(amount decimal.Decimal, midPrice decimal.Decimal, buyPrice decimal.Decimal) decimal.Decimal {
	if buyPrice.GreaterThan(midPrice) { // Refuse to lose
		midPrice = buyPrice
	}
	margin := midPrice.Mul(amount).Mul(margin)
	return midPrice.Add(midPrice.Mul(amount).Mul(feePerc).Round(2)).Add(margin)
}

func getBuyPrice(amount decimal.Decimal, midPrice decimal.Decimal) decimal.Decimal {
	margin := midPrice.Mul(amount).Mul(margin)
	return midPrice.Sub(midPrice.Mul(amount).Mul(feePerc).Round(2)).Add(margin)
}

func getCoinAmount(coinbase *coinbasepro.Client, coin string) string {
	accounts, err := coinbase.GetAccounts()
	if err != nil {
		println(err.Error())
	}
	for _, a := range accounts {
		if a.Currency == coin {
			return a.Balance
		}
	}
	return "0"
}

func placeOrder(coinbase *coinbasepro.Client, t string, market string, amount decimal.Decimal, price decimal.Decimal) {
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
		Println("Placed Order for " + market + " for " + amount.String() + " @ $" + price.String())
	}
}

type MarketData struct {
	market string
	buys   map[string]coinbasepro.Message
	sells  map[string]coinbasepro.Message
}