package gokalman

import (
	"fmt"
	"math"

	"github.com/gonum/matrix/mat64"
)

// NewInformation returns a new Information KF. To get the next estimate, call
// Update() with the next measurement and the control vector. This will return a
// new InformationEstimate which contains everything of this step and an error if any.
// Parameters:
// - i0: initial information state (usually a zero vector)
// - I0: initial information matrix (usually a zero matrix)
// - F: state update matrix
// - G: control matrix (if all zeros, then control vector will not be used)
// - H: measurement update matrix
// - noise: Noise
func NewInformation(i0 *mat64.Vector, I0 mat64.Symmetric, F, G, H mat64.Matrix, noise Noise) (*Information, *InformationEstimate, error) {
	// Let's check the dimensions of everything here to panic ASAP.
	if err := checkMatDims(i0, I0, "i0", "I0", rows2cols); err != nil {
		return nil, nil, err
	}
	if err := checkMatDims(F, I0, "F", "I0", rows2cols); err != nil {
		return nil, nil, err
	}
	if err := checkMatDims(H, i0, "H", "i0", cols2rows); err != nil {
		return nil, nil, err
	}

	// Populate with the initial values.
	rowsH, _ := H.Dims()
	Ir, _ := I0.Dims()
	I0Pred := mat64.NewSymDense(Ir, nil)
	est0 := NewInformationEstimate(i0, mat64.NewVector(rowsH, nil), I0, I0Pred)

	var Finv mat64.Dense
	if err := Finv.Inverse(mat64.DenseCopyOf(F)); err != nil {
		fmt.Printf("F *might* not invertible: %s\n", err)
	}

	var Qinv mat64.Dense
	if err := Qinv.Inverse(mat64.DenseCopyOf(noise.ProcessMatrix())); err != nil {
		fmt.Printf("Q *might* not invertible: %s\n", err)
	}
	var Rinv mat64.Dense
	if err := Rinv.Inverse(mat64.DenseCopyOf(noise.MeasurementMatrix())); err != nil {
		fmt.Printf("R *might* not invertible: %s\n", err)
	}

	return &Information{&Finv, G, H, &Qinv, &Rinv, noise, !IsNil(G), est0, est0, 0}, &est0, nil
}

// NewInformationFromState returns a new Information KF. To get the next estimate, call
// Update() with the next measurement and the control vector. This will return a
// new InformationEstimate which contains everything of this step and an error if any.
// Parameters:
// - i0: initial information state (usually a zero vector)
// - I0: initial information matrix (usually a zero matrix)
// - F: state update matrix
// - G: control matrix (if all zeros, then control vector will not be used)
// - H: measurement update matrix
// - noise: Noise
func NewInformationFromState(x0 *mat64.Vector, P0 mat64.Symmetric, F, G, H mat64.Matrix, noise Noise) (*Information, *InformationEstimate, error) {

	var I0 *mat64.SymDense
	var I0temp mat64.Dense
	if err := I0temp.Inverse(P0); err != nil {
		rI, _ := P0.Dims()
		I0 = mat64.NewSymDense(rI, nil)
		fmt.Printf("gokalman: initial covariance not invertible, using nil matrix: %s\n", err)
	} else {
		I0, _ = AsSymDense(&I0temp)
	}

	var i0 mat64.Vector
	i0.MulVec(I0, x0)

	return NewInformation(&i0, I0, F, G, H, noise)
}

// Information defines a vanilla kalman filter. Use NewVanilla to initialize.
type Information struct {
	Finv             mat64.Matrix
	G                mat64.Matrix
	H                mat64.Matrix
	Qinv             mat64.Matrix
	Rinv             mat64.Matrix
	Noise            Noise
	needCtrl         bool
	prevEst, initEst InformationEstimate
	step             int
}

func (kf *Information) String() string {
	return fmt.Sprintf("inv(F)=%v\nG=%v\nH=%v\n%s", mat64.Formatted(kf.Finv, mat64.Prefix("      ")), mat64.Formatted(kf.G, mat64.Prefix("  ")), mat64.Formatted(kf.H, mat64.Prefix("  ")), kf.Noise)
}

// GetStateTransition returns the F matrix.
// *WARNING:* Returns the *INVERSE* of F for the information filter.
func (kf *Information) GetStateTransition() mat64.Matrix {
	return kf.Finv
}

// GetInputControl returns the G matrix.
func (kf *Information) GetInputControl() mat64.Matrix {
	return kf.G
}

// GetMeasurementMatrix returns the H matrix.
func (kf *Information) GetMeasurementMatrix() mat64.Matrix {
	return kf.H
}

// SetStateTransition updates the F matrix.
func (kf *Information) SetStateTransition(F mat64.Matrix) {
	var Finv mat64.Dense
	if err := Finv.Inverse(mat64.DenseCopyOf(F)); err != nil {
		fmt.Printf("F *might* not invertible: %s\n", err)
	}
	kf.Finv = &Finv
}

// SetInputControl updates the G matrix.
func (kf *Information) SetInputControl(G mat64.Matrix) {
	kf.G = G
}

// SetMeasurementMatrix updates the H matrix.
func (kf *Information) SetMeasurementMatrix(H mat64.Matrix) {
	kf.H = H
}

// SetNoise updates the Noise.
func (kf *Information) SetNoise(n Noise) {
	kf.Noise = n
}

// GetNoise updates the F matrix.
func (kf *Information) GetNoise() Noise {
	return kf.Noise
}

// Reset reinitializes the KF with its initial estimate.
func (kf *Information) Reset() {
	kf.prevEst = kf.initEst
	kf.step = 0
	kf.Noise.Reset()
}

// Update implements the KalmanFilter interface.
func (kf *Information) Update(measurement, control *mat64.Vector) (est Estimate, err error) {
	if err = checkMatDims(control, kf.G, "control (u)", "G", rows2cols); kf.needCtrl && err != nil {
		return nil, err
	}

	if err = checkMatDims(measurement, kf.H, "measurement (y)", "H", rows2rows); err != nil {
		return nil, err
	}

	// zMat computation
	var zk mat64.Dense
	zk.Mul(kf.prevEst.infoMat, kf.Finv)
	zk.Mul(kf.Finv.T(), &zk)

	// Prediction step.
	// \hat{i}_{k+1}^{-}
	var zkzkqi mat64.Dense

	zkzkqi.Add(&zk, kf.Qinv)
	zkzkqi.Inverse(&zkzkqi)
	zkzkqi.Mul(&zk, &zkzkqi)
	zkzkqi.Scale(-1.0, &zkzkqi)
	rzk, _ := zkzkqi.Dims()
	var iKp1Minus, iKp1Minus1 mat64.Vector
	iKp1Minus.MulVec(kf.Finv.T(), kf.prevEst.infoState)
	if kf.needCtrl {
		iKp1Minus1.MulVec(kf.G, control)
		iKp1Minus1.MulVec(&zk, &iKp1Minus1)
		iKp1Minus.AddVec(&iKp1Minus, &iKp1Minus1)
	}
	var iKp1MinusM mat64.Dense
	iKp1MinusM.Add(Identity(rzk), &zkzkqi)
	iKp1Minus.MulVec(&iKp1MinusM, &iKp1Minus)

	// I_{k+1}^{-}
	var Ikp1Minus mat64.Dense
	Ikp1Minus.Mul(&zkzkqi, zk.T())
	Ikp1Minus.Add(&zk, &Ikp1Minus)

	var ykHat mat64.Vector
	ykHat.MulVec(kf.H, kf.prevEst.State())
	ykHat.AddVec(&ykHat, kf.Noise.Measurement(kf.step))

	// Measurement update
	var HTR mat64.Dense
	if rR, cR := kf.Rinv.Dims(); rR == 1 && cR == 1 {
		// Rinv is a scalar and mat64 won't be happy.
		HTR.Scale(kf.Rinv.At(0, 0), kf.H.T())
	} else {
		HTR.Mul(kf.H.T(), kf.Rinv)
	}

	var ikp1Plus mat64.Vector
	ikp1Plus.MulVec(&HTR, measurement)
	ikp1Plus.AddVec(&ikp1Plus, &iKp1Minus)

	// I_{k+1}^{+}
	var Ikp1Plus mat64.Dense
	Ikp1Plus.Mul(&HTR, kf.H)
	Ikp1Plus.Add(&Ikp1Minus, &Ikp1Plus)

	Ikp1MinusSym, err := AsSymDense(&Ikp1Minus)
	if err != nil {
		panic(err)
	}

	Ikp1PlusSym, err := AsSymDense(&Ikp1Plus)
	if err != nil {
		panic(err)
	}
	est = NewInformationEstimate(&ikp1Plus, &ykHat, Ikp1PlusSym, Ikp1MinusSym)
	kf.prevEst = est.(InformationEstimate)
	kf.step++
	return
}

// InformationEstimate is the output of each update state of the Information KF.
// It implements the Estimate interface.
type InformationEstimate struct {
	infoState, meas              *mat64.Vector
	infoMat, predInfoMat         mat64.Symmetric
	cachedState                  *mat64.Vector
	cachedCovar, predCachedCovar mat64.Symmetric
}

// IsWithinNσ returns whether the estimation is within the 2σ bounds.
func (e InformationEstimate) IsWithinNσ(N float64) bool {
	state := e.State()
	covar := e.Covariance()
	for i := 0; i < state.Len(); i++ {
		nσ := N * math.Sqrt(covar.At(i, i))
		if state.At(i, 0) > nσ || state.At(i, 0) < -nσ {
			return false
		}
	}
	return true
}

// IsWithin2σ returns whether the estimation is within the 2σ bounds.
func (e InformationEstimate) IsWithin2σ() bool {
	return e.IsWithinNσ(2)
}

// State implements the Estimate interface.
func (e InformationEstimate) State() *mat64.Vector {
	if e.cachedState == nil {
		rState, _ := e.infoState.Dims()
		e.cachedState = mat64.NewVector(rState, nil)
		e.cachedState.MulVec(e.Covariance(), e.infoState)
	}
	return e.cachedState
}

// Measurement implements the Estimate interface.
func (e InformationEstimate) Measurement() *mat64.Vector {
	return e.meas
}

// Innovation implements the Estimate interface.
func (e InformationEstimate) Innovation() *mat64.Vector {
	return e.infoState
}

// Covariance implements the Estimate interface.
// *NOTE:* With the IF, one cannot view the covariance matrix until there is enough information.
func (e InformationEstimate) Covariance() mat64.Symmetric {
	if e.cachedCovar == nil {
		rCovar, _ := e.infoMat.Dims()
		e.cachedCovar = mat64.NewSymDense(rCovar, nil)
		infoMat := mat64.DenseCopyOf(e.infoMat)
		var tmpCovar mat64.Dense
		err := tmpCovar.Inverse(infoMat)
		if err != nil {
			fmt.Printf("gokalman: InformationEstimate: information matrix is not (yet) invertible: %s\n", err)
			return e.cachedCovar
		}
		cachedCovar, _ := AsSymDense(&tmpCovar)
		e.cachedCovar = cachedCovar
	}
	return e.cachedCovar
}

// PredCovariance implements the Estimate interface.
// *NOTE:* With the IF, one cannot view the prediction covariance matrix until there is enough information.
func (e InformationEstimate) PredCovariance() mat64.Symmetric {
	if e.predCachedCovar == nil {
		rCovar, _ := e.predInfoMat.Dims()
		e.predCachedCovar = mat64.NewSymDense(rCovar, nil)
		predInfoMat := mat64.DenseCopyOf(e.predInfoMat)
		var tmpCovar mat64.Dense
		err := tmpCovar.Inverse(predInfoMat)
		if err != nil {
			fmt.Printf("gokalman: InformationEstimate: prediction information matrix is not (yet) invertible: %s\n", err)
			return e.predCachedCovar
		}
		predCachedCovar, err := AsSymDense(&tmpCovar)
		if err != nil {
			fmt.Printf("gokalman: InformationEstimate: prediction covariance matrix: %s\n", err)
			return e.predCachedCovar
		}
		e.predCachedCovar = predCachedCovar
	}
	return e.predCachedCovar
}

func (e InformationEstimate) String() string {
	state := mat64.Formatted(e.State(), mat64.Prefix("  "))
	meas := mat64.Formatted(e.Measurement(), mat64.Prefix("  "))
	covar := mat64.Formatted(e.Covariance(), mat64.Prefix("  "))
	innov := mat64.Formatted(e.Innovation(), mat64.Prefix("  "))
	predp := mat64.Formatted(e.PredCovariance(), mat64.Prefix("   "))
	return fmt.Sprintf("{\ns=%v\ny=%v\nP=%v\nP-=%v\ni=%v\n}", state, meas, covar, predp, innov)
}

// NewInformationEstimate initializes a new InformationEstimate.
func NewInformationEstimate(infoState, meas *mat64.Vector, infoMat, predInfoMat mat64.Symmetric) InformationEstimate {
	return InformationEstimate{infoState, meas, infoMat, predInfoMat, nil, nil, nil}
}
