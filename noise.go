package gokalman

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gonum/matrix/mat64"
	"github.com/gonum/stat/distmv"
)

// Noise allows to handle the noise for a KF.
type Noise interface {
	Process(k int) *mat64.Vector        // Returns the process noise w at step k
	Measurement(k int) *mat64.Vector    // Returns the measurement noise w at step k
	ProcessMatrix() mat64.Symmetric     // Returns the process noise matrix Q
	MeasurementMatrix() mat64.Symmetric // Returns the measurement noise matrix R
}

// Noiseless is noiseless and implements the Noise interface.
type Noiseless struct {
	processSize     int
	measurementSize int
}

// Process returns a vector of the correct size.
func (n Noiseless) Process(k int) *mat64.Vector {
	return mat64.NewVector(n.processSize, nil)
}

// Measurement returns a vector of the correct size.
func (n Noiseless) Measurement(k int) *mat64.Vector {
	return mat64.NewVector(n.measurementSize, nil)
}

// ProcessMatrix implements the Noise interface.
func (n Noiseless) ProcessMatrix() mat64.Symmetric {
	return mat64.NewSymDense(n.processSize, nil)
}

// MeasurementMatrix implements the Noise interface.
func (n Noiseless) MeasurementMatrix() mat64.Symmetric {
	return mat64.NewSymDense(n.measurementSize, nil)
}

// BatchNoise implements the Noise interface.
type BatchNoise struct {
	process     []*mat64.Vector // Array of process noise
	measurement []*mat64.Vector // Array of process noise
}

// Process implements the Noise interface.
func (n BatchNoise) Process(k int) *mat64.Vector {
	if k >= len(n.process) {
		panic(fmt.Errorf("no process noise defined at step k=%d", k))
	}
	return n.process[k]
}

// Measurement implements the Noise interface.
func (n BatchNoise) Measurement(k int) *mat64.Vector {
	if k >= len(n.measurement) {
		panic(fmt.Errorf("no measurement noise defined at step k=%d", k))
	}
	return n.measurement[k]
}

// ProcessMatrix implements the Noise interface.
func (n BatchNoise) ProcessMatrix() mat64.Symmetric {
	rows, _ := n.process[0].Dims()
	return mat64.NewSymDense(rows, nil)
}

// MeasurementMatrix implements the Noise interface.
func (n BatchNoise) MeasurementMatrix() mat64.Symmetric {
	rows, _ := n.measurement[0].Dims()
	return mat64.NewSymDense(rows, nil)
}

// AWGN implements the Noise interface and generates an Additive white Gaussian noise.
type AWGN struct {
	Q, R        mat64.Symmetric
	process     *distmv.Normal
	measurement *distmv.Normal
}

// NewAWGN creates new AWGN noise from the provided Q and R.
func NewAWGN(Q, R mat64.Symmetric) *AWGN {
	randomSeed := rand.New(rand.NewSource(time.Now().UnixNano()))
	sizeQ, _ := Q.Dims()
	process, ok := distmv.NewNormal(make([]float64, sizeQ), Q, randomSeed)
	if !ok {
		panic("process noise invalid")
	}
	sizeR, _ := R.Dims()
	meas, ok := distmv.NewNormal(make([]float64, sizeR), R, randomSeed)
	if !ok {
		panic("measurement noise invalid")
	}
	return &AWGN{Q, R, process, meas}
}

// ProcessMatrix implements the Noise interface.
func (n AWGN) ProcessMatrix() mat64.Symmetric {
	return n.Q
}

// MeasurementMatrix implements the Noise interface.
func (n AWGN) MeasurementMatrix() mat64.Symmetric {
	return n.R
}

// Process implements the Noise interface.
func (n AWGN) Process(k int) *mat64.Vector {
	r := n.process.Rand(nil)
	return mat64.NewVector(len(r), r)
}

// Measurement implements the Noise interface.
func (n AWGN) Measurement(k int) *mat64.Vector {
	r := n.measurement.Rand(nil)
	return mat64.NewVector(len(r), r)
}