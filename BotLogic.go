package main

import (
	"database/sql"
	. "fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/preichenberger/go-coinbasepro/v2"
	"strconv"
)

func run(coinbase *coinbasepro.Client, settings BotSettings, sql *sql.DB, discord *discordgo.Session) {
	BotLog(discord, settings.Name+" Bot Starting on '"+settings.Market+"'")
	Println(settings.Name + " Bot Starting on '" + settings.Market + "'")
	//startPoint := int64(1437487200)
	//hourlyPoints := computePoints(sql, startPoint, startPoint + (60 * 60 * 60), settings)
	//halfDayPoints := computePoints(sql, startPoint, startPoint + (60 * 60 * 60 * 12), settings)
	//dayPoints := computePoints(sql, startPoint, startPoint + (60 * 60 * 60 * 24), settings)

}

// Compute the best times to buy / sell based on a given set of start and end points / entries
func computePoints(sql *sql.DB, startPoint int64, endPoint int64, settings BotSettings) []float64 {
	increments := (endPoint - startPoint) / 60 // Amount of entries
	// SELECT * FROM market_data WHERE market='BTC-USD' AND timestamp BETWEEN '1437487200' AND '1437489000';
	queryRows, err := sql.Query("SELECT * FROM market_data WHERE market='" + settings.Market + "' AND timestamp BETWEEN '" +
		strconv.FormatInt(startPoint-1, 10) + "' AND '" + strconv.FormatInt(endPoint+1, 10) + "';")
	if err != nil {
		println(err.Error())
		return make([]float64, 0)
	}
	// Collect entries from query into array
	history := make([]HistoricalEntry, 0)
	for queryRows.Next() {
		var entry HistoricalEntry
		if err := queryRows.Scan(&entry.exchange, &entry.market, &entry.timestamp, &entry.lowestPrice, &entry.highestPrice, &entry.firstTradePrice, &entry.lastTradePrice, &entry.volume); err != nil {
			println(err.Error())
			return make([]float64, 0)
		}
		history = append(history, entry)
	}
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
