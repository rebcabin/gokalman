package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"sync"

	"github.com/ChristopherRabotin/gokalman"
	"github.com/gonum/matrix/mat64"
)

func main() {
	// Load CSV measurement file.
	var control []*mat64.Vector
	if f, err := os.Open("uvec.csv"); err != nil {
		panic(err)
	} else {
		// Create a new reader.
		r := csv.NewReader(bufio.NewReader(f))
		for {
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			value, err := strconv.ParseFloat(record[0], 64)
			if err != nil {
				fmt.Printf("could not parse float: %s", err)
				continue
			}
			control = append(control, mat64.NewVector(1, []float64{value}))
		}
	}

	// Both yacchist and yposthist are single line CSV files.
	singleRecord := func(name string) []float64 {
		var dest []float64
		if f, err := os.Open(name); err != nil {
			panic(err)
		} else {
			// Create a new reader.
			r := csv.NewReader(bufio.NewReader(f))
			record, err := r.Read()
			if err == io.EOF {
				return dest
			}

			for _, svalue := range record {
				value, err := strconv.ParseFloat(svalue, 64)
				if err != nil {
					fmt.Printf("could not parse float: %s", err)
					continue
				}
				if math.IsNaN(value) {
					value = 0
				}
				dest = append(dest, value)
			}
		}
		return dest
	}

	yacc := singleRecord("yacchist.csv")
	ypos := singleRecord("yposhist.csv")

	// Prepare the estimate channel.
	var wg sync.WaitGroup
	estimateChan := make(chan (gokalman.Estimate), 1)
	go func() {
		wg.Add(1)
		ce, _ := gokalman.NewCSVExporter([]string{"position", "velocity", "acceleration"}, ".", "vanilla.csv")
		for {
			est, more := <-estimateChan
			if !more {
				ce.Close()
				wg.Done()
				break
			}
			ce.Write(est)
		}
	}()

	// DT system
	//Δt := 0.01
	F := mat64.NewDense(4, 4, []float64{1, 0.01, 0.0001, 0, 0, 1, 0.01, 0, 0, 0, 1, 0, 0, 0, 0, 1.0005})
	G := mat64.NewDense(4, 1, []float64{0.0, 0.0001, 0.01, 0.0})
	// Note that we will be using two difference H matrices, which we'll swap on the fly.
	H1 := mat64.NewDense(2, 4, []float64{1, 0, 0, 0, 0, 0, 1, 1})
	H2 := mat64.NewDense(2, 4, []float64{0, 0, 0, 0, 0, 0, 1, 1})
	// Noise
	Q := mat64.NewSymDense(4, []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0.005e-3, 0, 0, 0, 0, 0.5303e-3})
	R := mat64.NewSymDense(2, []float64{0.005, 0, 0, 0.0005})

	// Vanilla KF
	noise := gokalman.NewNoiseless(Q, R)
	x0 := mat64.NewVector(4, []float64{0, 0.35, 0, 0})
	Covar0 := gokalman.ScaledIdentity(4, 10)
	kf, err := gokalman.NewVanilla(x0, Covar0, F, G, H2, noise)
	fmt.Printf("Vanilla: \n%s", kf)
	if err != nil {
		panic(err)
	}

	//for k, yaccK := range yacc {
	k := 0
	yaccK := yacc[k]
	measurement := mat64.NewVector(2, []float64{ypos[k], yaccK})
	if k%10 == 0 {
		// Switch to using H1
		kf.H = H1
	}
	newEstimate, err := kf.Update(measurement, control[k])
	if k%10 == 0 {
		// Switch back to using H2
		kf.H = H2
	}
	if err != nil {
		panic(err)
	}
	estimateChan <- newEstimate
	//}
	close(estimateChan)

	wg.Wait()
}
