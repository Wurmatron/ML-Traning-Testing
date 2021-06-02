package main

import (
	"database/sql"
	. "fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/preichenberger/go-coinbasepro/v2"
	"math/rand"
	"strconv"
	"time"
)

// ML Data
const botCount = 100

var generation = 0
var bestBot NeuralNet
var bots []NeuralNet
var bestFitness = 0.0

// Training data
const generationTimeframe = 24

func run(coinbase *coinbasepro.Client, settings BotSettings, sql *sql.DB, discord *discordgo.Session) {
	BotLog(discord, settings.Name+" Bot Starting on '"+settings.Market+"'")
	Println(settings.Name + " Bot Starting on '" + settings.Market + "'")
	startPoint := getMarketStartingPoint(sql, settings.Market)
	// Setup ML
	bots := createRandomBots()
	for {
		bots = runGeneration(discord, sql, startPoint, settings, bots)
		generation++
	}
}

func runGeneration(discord *discordgo.Session, sql *sql.DB, start int64, settings BotSettings, bots []NeuralNet) []NeuralNet {
	// Compute Bot Scoring
	history := getHistory(sql, start, start+(60*60*60), settings.Market)
	hourlyPoints := computePoints(sql, start, start+(60*60*60), settings)
	botScores := make([]BotGenerationScore, 0)
	botChannels := make([]chan BotGenerationScore, len(bots))
	for x := 0; x < len(botChannels); x++ {
		botChannels[x] = make(chan BotGenerationScore)
	}
	// Start Bot Calculations
	for index, bot := range bots {
		go runBotForGeneration(bot, history, hourlyPoints, botChannels[index])
	}
	// Collect bot calculations
	for _, channel := range botChannels {
		botScore := <-channel
		botScores = append(botScores, botScore)
	}
	// Generation Scoring
	bestOfGenerationScore := -1000000.0
	bestGenerationBot := bots[0]
	generationalAvg := 0.0
	for _, botScore := range botScores {
		if botScore.score > bestOfGenerationScore {
			bestOfGenerationScore = botScore.score
			bestGenerationBot = botScore.Bot
		}
		generationalAvg = generationalAvg + botScore.score
	}
	// Check for best score
	if bestOfGenerationScore > bestFitness || bestBot.HiddenLayers == nil {
		bestFitness = bestOfGenerationScore
		bestBot = bestGenerationBot
	}
	generationalAvg = generationalAvg / botCount
	// Display Info
	generationInformational := Sprintf("Generation %s  Gen: %.8f Best: %.8f Avg %.8f \n", strconv.Itoa(generation), bestOfGenerationScore, bestFitness, generationalAvg)
	BotLog(discord, generationInformational)
	Printf(generationInformational)
	// Setup Next Generation
	topBots := getTop(botScores, botCount/10)
	newBotsNeeded := botCount - len(topBots) - (botCount / 10)
	bots = make([]NeuralNet, 0)
	bots = append(bots, topBots...)
	// Mutate to fill missing bots
	for x := 0; x < newBotsNeeded; x++ {
		rand.Seed(time.Now().UnixNano())
		randBot := rand.Intn(len(topBots))
		rand.Seed(time.Now().UnixNano())
		bots = append(bots, mutate(topBots[randBot], 10+rand.Intn(30)))
	}
	for x := 0; x < (botCount / 10); x++ {
		bots = append(bots, RandomNet(14, 3, []int{12, 12, 12}, 13))
	}
	return bots
}

// Score a bot over the given history
func scoreBot(net NeuralNet, history []HistoricalEntry, scoring []float64) float64 {
	score := 0.0
	for index, entry := range history {
		netOutput := Compute(convertToNeural(entry), net) // 0 Nothing, 1 Buy, 2 Sell
		marketScore := scoring[index]
		score = score + computeBotScore(netOutput, marketScore)
	}
	return score
}

func runBotForGeneration(net NeuralNet, history []HistoricalEntry, scoring []float64, channel chan BotGenerationScore) {
	score := BotGenerationScore{
		Bot:   net,
		score: scoreBot(net, history, scoring),
	}
	channel <- score
}

// Calculate the score of the bots actions
func computeBotScore(netOutput []float64, marketScore float64) float64 {
	score := 0.0
	score = score - netOutput[0]*marketScore // Chance to be doing nothing
	if marketScore > .8 {                    // Time to sell
		score = score + (netOutput[2] * marketScore)
		score = score - (netOutput[1] * marketScore)
	} else if marketScore < .2 { // Time to buy
		score = score + (netOutput[1] * (1 - marketScore))
		score = score - (netOutput[2] * marketScore)
	} else { // Should be waiting
		score = score - (netOutput[1] * marketScore)
		score = score - (netOutput[2] * marketScore)
	}
	return score
}

// Converts the history into something a neural net can understand, (0 - 1)
func convertToNeural(entry HistoricalEntry) []float64 {
	neueral := make([]float64, 13)
	neueral[0] = sigmoid(entry.firstTradePrice / 10000)
	neueral[1] = sigmoid(entry.lastTradePrice / 10000)
	neueral[2] = sigmoid(entry.highestPrice / 10000)
	neueral[3] = sigmoid(entry.lowestPrice / 10000)
	neueral[4] = sigmoid(entry.volume / 10000)
	return neueral
}

// Creates a fully new set of bots with random values
func createRandomBots() []NeuralNet {
	bots := make([]NeuralNet, botCount)
	for index := 0; index < botCount; index++ {
		bots[index] = RandomNet(14, 3, []int{12, 12, 12}, 13)
	}
	return bots
}

// Get the earliest point of the markets history, for training
func getMarketStartingPoint(sql *sql.DB, market string) int64 {
	query, err := sql.Query("SELECT MIN(timestamp) FROM market_data WHERE market='" + market + "'")
	if err != nil {
		println(err.Error())
	}
	firstTimestamp := int64(0)
	if query != nil && query.Next() {
		err = query.Scan(&firstTimestamp)
		if err != nil {
			println(err.Error())
		}
	}
	return firstTimestamp
}

// Compute the best times to buy / sell based on a given set of start and end points / entries
func computePoints(sql *sql.DB, startPoint int64, endPoint int64, settings BotSettings) []float64 {
	increments := (endPoint - startPoint) / 60 // Amount of entries
	history := getHistory(sql, startPoint, endPoint, settings.Market)
	points := make([]float64, len(history))
	// Find prices
	diffIncrement := 1.0 / float64(increments)
	currentHighestScore := 1.0
	currentLowestScore := 0.0
	for {
		// Check for end of sorting
		count := 0
		for _, entry := range history {
			if entry.lowestPrice == 0 && entry.highestPrice == 0 {
				count++
			}
		}
		if count == len(history) {
			break
		}
		lowHigh := findHighAndLow(history)
		// Add to points
		points[lowHigh[0]] = currentLowestScore
		points[lowHigh[1]] = currentHighestScore
		// Set price to 0, ignored by lowHigh Sorting
		history[lowHigh[0]].lowestPrice = 0
		history[lowHigh[0]].highestPrice = 0
		history[lowHigh[1]].lowestPrice = 0
		history[lowHigh[1]].highestPrice = 0
		// Update scores
		currentHighestScore = currentHighestScore - diffIncrement
		currentLowestScore = currentLowestScore + diffIncrement
	}
	return points
}

// Gets the historical data for the given time peroid
func getHistory(sql *sql.DB, startPoint int64, endPoint int64, market string) []HistoricalEntry {
	// SELECT * FROM market_data WHERE market='BTC-USD' AND timestamp BETWEEN '1437487200' AND '1437489000';
	queryRows, err := sql.Query("SELECT * FROM market_data WHERE market='" + market + "' AND timestamp BETWEEN '" +
		strconv.FormatInt(startPoint-1, 10) + "' AND '" + strconv.FormatInt(endPoint+1, 10) + "';")
	if err != nil {
		println(err.Error())
		return make([]HistoricalEntry, 0)
	}
	// Collect entries from query into array
	history := make([]HistoricalEntry, 0)
	for queryRows.Next() {
		var entry HistoricalEntry
		if err := queryRows.Scan(&entry.exchange, &entry.market, &entry.timestamp, &entry.lowestPrice, &entry.highestPrice, &entry.firstTradePrice, &entry.lastTradePrice, &entry.volume); err != nil {
			println(err.Error())
			return make([]HistoricalEntry, 0)
		}
		history = append(history, entry)
	}
	return history
}

// Returns the index of the lowest and highest entries [low, high]
func findHighAndLow(entries []HistoricalEntry) []int {
	lowestIndex := 0
	lowestPrice := entries[0].lowestPrice
	highestIndex := 0
	highestPrice := entries[0].highestPrice
	for index, entry := range entries {
		if entry.lowestPrice == 0 && entry.highestPrice == 0 { // Entry has been sorted
			continue
		}
		// Check for new lowest price
		if lowestPrice > entry.lowestPrice {
			lowestIndex = index
			lowestPrice = entry.lowestPrice
		}
		// Check for new highest price
		if highestPrice < entry.highestPrice {
			highestIndex = index
			highestPrice = entry.highestPrice
		}
	}
	lowHigh := make([]int, 2)
	lowHigh[0] = lowestIndex
	lowHigh[1] = highestIndex
	return lowHigh
}

func mutate(net NeuralNet, mutationCount int) NeuralNet {
	for x := 0; x < mutationCount; x++ {
		rand.Seed(time.Now().UnixNano())
		randLayer := rand.Intn(len(net.HiddenLayers))
		randNeuron := 0
		if randLayer > len(net.HiddenLayers)-1 {
			rand.Seed(time.Now().UnixNano())
			randNeuron = rand.Intn(len(net.OutputLayer))
		} else {
			randNeuron = rand.Intn(len(net.HiddenLayers))
		}
		if randLayer > len(net.HiddenLayers) { // Output Layer
			net.OutputLayer[randNeuron] = mutateNeuron(net.OutputLayer[randNeuron])
		} else { // Hidden Layers
			net.HiddenLayers[randLayer][randNeuron] = mutateNeuron(net.HiddenLayers[randLayer][randNeuron])
		}
	}
	return net
}

func mutateNeuron(neuron Neuron) Neuron {
	randSel := rand.Intn(3)
	rand.Seed(time.Now().UnixNano())
	addOrSub := rand.Intn(1)
	rand.Seed(time.Now().UnixNano())
	if randSel == 0 { // Activation
		if addOrSub == 1 {
			neuron.Activation += rand.Float64()
		} else {
			neuron.Activation -= rand.Float64()
		}
	} else if randSel == 1 { // Bias
		if addOrSub == 1 {
			neuron.Bias += rand.Float64()
		} else {
			neuron.Bias -= rand.Float64()
		}
	} else {
		weight := rand.Intn(len(neuron.Weights))
		if addOrSub == 1 {
			neuron.Weights[weight] += rand.Float64() * 5
		} else {
			neuron.Weights[weight] -= rand.Float64() * 5
		}
	}
	return neuron
}

func getTop(botScores []BotGenerationScore, count int) []NeuralNet {
	topNets := make([]NeuralNet, 0)
	for x := 0; x < count; x++ {
		// Find best bot in current net array
		bestIndex := 0
		bestScore := -999999999.0
		for index := 0; index < len(botScores); index++ {
			if botScores[index].score > bestScore {
				bestIndex = index
				bestScore = botScores[index].score
			}
		}
		botScores[bestIndex].score = -1
		topNets = append(topNets, botScores[bestIndex].Bot)
	}
	return topNets
}
