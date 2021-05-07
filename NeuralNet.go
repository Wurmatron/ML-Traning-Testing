package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

type Neuron struct {
	Bias       float64
	Activation float64
	Weights    []float64
}

type NeuralNet struct {
	HiddenLayers [][]Neuron
	OutputLayer  []Neuron
}

func Compute(input []float64, net NeuralNet) []float64 {
	// Calculate each layer
	for layer := 0; layer < len(net.HiddenLayers)+1; layer++ {
		input = calculateLayer(input, layer, net)
	}
	return input
}

func calculateLayer(activation []float64, layer int, net NeuralNet) []float64 {
	layerNeurons := getLayerNeurons(layer, net)
	output := make([]float64, len(layerNeurons))
	for index := 0; index < len(layerNeurons); index++ {
		total := 0.0
		for weight := 0; weight < len(layerNeurons[index].Weights); weight++ {
			total += layerNeurons[index].Weights[weight] * activation[weight]
		}
		output[index] = sigmoid(total + layerNeurons[index].Bias)
	}
	return output
}

func sigmoid(num float64) float64 {
	return 1 / (1 + math.Exp(-num))
}

func getLayerNeurons(layer int, net NeuralNet) []Neuron {
	if layer > len(net.HiddenLayers) {
		return net.HiddenLayers[layer]
	} else {
		return net.OutputLayer
	}
}

func RandomNet(inputSize int, hiddenLayerCount int, hiddenLayer []int, outputLayerSize int) NeuralNet {
	if hiddenLayerCount != len(hiddenLayer) {
		fmt.Println("Invalid Neural-Net Config")
		return NeuralNet{}
	}
	outputLayer := make([]Neuron, outputLayerSize)
	hiddenLayers := make([][]Neuron, hiddenLayerCount)
	for index := 0; index < outputLayerSize; index++ {
		outputLayer[index] = RandomNeuron(hiddenLayer[len(hiddenLayer)-1], 5.0)
	}
	for layer := 0; layer < hiddenLayerCount; layer++ {
		hiddenLayers[layer] = make([]Neuron, hiddenLayer[layer])
		for index := 0; index < hiddenLayer[layer]; index++ {
			if layer == 0 {
				hiddenLayers[layer][index] = RandomNeuron(inputSize, 5.0)
			} else {
				hiddenLayers[layer][index] = RandomNeuron(hiddenLayer[layer], 5.0)
			}
		}
	}
	return NeuralNet{
		HiddenLayers: hiddenLayers,
		OutputLayer:  outputLayer,
	}
}

func RandomNeuron(weightsCount int, highestWeight float64) Neuron {
	weights := make([]float64, weightsCount)
	rand.Seed(time.Now().UnixNano() - int64(rand.Int())) // Yep you see that correctly
	for index := 0; index < weightsCount; index++ {
		weights[index] = rand.Float64() * highestWeight
	}
	return Neuron{
		Bias:       rand.Float64() * highestWeight,
		Activation: rand.Float64() * highestWeight,
		Weights:    weights,
	}
}
