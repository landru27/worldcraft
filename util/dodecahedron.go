package main

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  import necessary external packages  ///////////////////////////////////////////////////////////////////////////////////////

import (
	"flag"
	"fmt"
	"math"
	"os"
)

type Vertex struct {
	X float32
	Y float32
	Z float32
}

func (v *Vertex) Define(x float32, y float32, z float32) {
	v.X = x
	v.Y = y
	v.Z = z
}

type Facet struct {
	X float32
	Y float32
	Z float32
	D float32
}

func (f *Facet) Define(x float32, y float32, z float32, d float32) {
	f.X = x
	f.Y = y
	f.Z = z
	f.D = d
}

func main() {
	//var err error

	var step float32
	var scale float32

	step64 := flag.Float64("step", 1, "the step-size when examining points; acts like precision")
	scale64 := flag.Float64("scale", 1, "the scale factor when extrapolating results; acts like amplitude")
	rotateX := flag.Float64("rotateX", 0, "rotation in degrees around the X-axis")
	rotateY := flag.Float64("rotateY", 0, "rotation in degrees around the Y-axis")
	rotateZ := flag.Float64("rotateZ", 0, "rotation in degrees around the Z-axis")
	flag.Parse()

	step = float32(*step64)
	scale = float32(*scale64)
	*rotateX = *rotateX * (math.Pi / 180)
	*rotateY = *rotateY * (math.Pi / 180)
	*rotateZ = *rotateZ * (math.Pi / 180)

	var phi = float32(math.Phi)
	var phiinv = 1 / phi
	var phisqr = phi * phi
	fmt.Printf("constants : phi, 1/phi, phi^2 : %v, %v, %v\n", phi, phiinv, phisqr)
	fmt.Printf("\n")

	var vertices [20]Vertex

	vertices[0].Define( 1,  1,  1)
	vertices[1].Define( 1,  1, -1)
	vertices[2].Define( 1, -1,  1)
	vertices[3].Define( 1, -1, -1)
	vertices[4].Define(-1,  1,  1)
	vertices[5].Define(-1,  1, -1)
	vertices[6].Define(-1, -1,  1)
	vertices[7].Define(-1, -1, -1)

	vertices[8].Define(  0,  phi,  phiinv)
	vertices[9].Define(  0,  phi, -phiinv)
	vertices[10].Define( 0, -phi,  phiinv)
	vertices[11].Define( 0, -phi, -phiinv)
	vertices[12].Define( phiinv,  0,  phi)
	vertices[13].Define(-phiinv,  0,  phi)
	vertices[14].Define( phiinv,  0, -phi)
	vertices[15].Define(-phiinv,  0, -phi)
	vertices[16].Define( phi,  phiinv,  0)
	vertices[17].Define( phi, -phiinv,  0)
	vertices[18].Define(-phi,  phiinv,  0)
	vertices[19].Define(-phi, -phiinv,  0)

	for _, v := range(vertices) {
		fmt.Printf("vertex    : x, y, z : %v, %v, %v\n", v.X, v.Y, v.Z)
	}
	fmt.Printf("\n")

	var facets [12]Facet

	facets[0].Define(  phi,   1,   0,  phisqr)
	facets[1].Define(  phi,   1,   0, -phisqr)
	facets[2].Define(  phi,  -1,   0,  phisqr)
	facets[3].Define(  phi,  -1,   0, -phisqr)
	facets[4].Define(  0,   phi,   1,  phisqr)
	facets[5].Define(  0,   phi,   1, -phisqr)
	facets[6].Define(  0,   phi,  -1,  phisqr)
	facets[7].Define(  0,   phi,  -1, -phisqr)
	facets[8].Define(  1,     0, phi,  phisqr)
	facets[9].Define(  1,     0, phi, -phisqr)
	facets[10].Define(-1,     0, phi,  phisqr)
	facets[11].Define(-1,     0, phi, -phisqr)

	var indxX float32
	var indxY float32
	var indxZ float32
	var vectors [12]int

	var min = int(-2 * scale)
	var max = int( 2 * scale)
	var rng = int( 4 * scale)

	var blocksReg []bool
	blocksReg = make([]bool, (int(4 * int(scale)) * int(4 * int(scale)) * int(4 * int(scale))))
	var blocksRot []bool
	blocksRot = make([]bool, (int(4 * int(scale)) * int(4 * int(scale)) * int(4 * int(scale))))

	cosa := float32(math.Cos(*rotateX));
	sina := float32(math.Sin(*rotateX));
	cosb := float32(math.Cos(*rotateY));
	sinb := float32(math.Sin(*rotateY));
	cosc := float32(math.Cos(*rotateZ));
	sinc := float32(math.Sin(*rotateZ));

	Axx := cosa * cosb;
	Axy := cosa * sinb * sinc - sina * cosc;
	Axz := cosa * sinb * cosc + sina * sinc;

	Ayx := sina * cosb;
	Ayy := sina * sinb * sinc + cosa * cosc;
	Ayz := sina * sinb * cosc - cosa * sinc;

	Azx := -sinb;
	Azy := cosb * sinc;
	Azz := cosb * cosc;

	var rotX float32
	var rotY float32
	var rotZ float32

	for indxX = -2; indxX <= 2; indxX += step {
		for indxY = -2; indxY <= 2; indxY += step {
			for indxZ = -2; indxZ <= 2; indxZ += step {

				for indxF, f := range(facets) {
					vec := (indxX * f.X) + (indxY * f.Y) + (indxZ * f.Z) + f.D

					if vec  > 0 { vectors[indxF] =  1 }
					if vec == 0 { vectors[indxF] =  0 }
					if vec  < 0 { vectors[indxF] = -1 }
				}

				if ((vectors[0]  >= 0) &&
				    (vectors[1]  <= 0) &&
				    (vectors[2]  >= 0) &&
				    (vectors[3]  <= 0) &&
				    (vectors[4]  >= 0) &&
				    (vectors[5]  <= 0) &&
				    (vectors[6]  >= 0) &&
				    (vectors[7]  <= 0) &&
				    (vectors[8]  >= 0) &&
				    (vectors[9]  <= 0) &&
				    (vectors[10] >= 0) &&
				    (vectors[11] <= 0)) {
					indxB := ((int(indxX * scale) + max) * (rng * rng)) +
						 ((int(indxY * scale) + max) *  rng       ) +
						 ((int(indxZ * scale) + max)              )
					blocksReg[indxB] = true;

					rotX = Axx * indxX + Axy * indxY + Axz * indxZ;
					rotY = Ayx * indxX + Ayy * indxY + Ayz * indxZ;
					rotZ = Azx * indxX + Azy * indxY + Azz * indxZ;

					indxR := ((int(rotX * scale) + max) * (rng * rng)) +
						 ((int(rotY * scale) + max) *  rng       ) +
						 ((int(rotZ * scale) + max)              )
					blocksRot[indxR] = true
				}
			}
		}
	}

	var iX int
	var iY int
	var iZ int
	var iB int

	for iX = min; iX < max; iX += 1 {
		for iY = min; iY < max; iY += 1 {
			for iZ = min; iZ < max; iZ += 1 {

				iB = ((iX + max) * (rng * rng)) +
				     ((iY + max) *  rng       ) +
				     ((iZ + max)              )
				if blocksReg[iB] == true {
					fmt.Printf("block     : x, y, z : %v, %v, %v\n", iX, iY, iZ)
				}
			}
		}
	}
	fmt.Printf("\n")

	for iZ = min; iZ < max; iZ += 1 {
		for iY = min; iY < max; iY += 1 {
			fmt.Printf("    ");
			for iX = min; iX < max; iX += 1 {

				iB = ((iX + max) * (rng * rng)) +
				     ((iY + max) *  rng       ) +
				     ((iZ + max)              )
				if blocksRot[iB] == true {
					fmt.Printf("# ")
				} else {
					fmt.Printf(". ")
				}
			}
			fmt.Printf("\n");
		}
	fmt.Printf("    --\n");
	}

	os.Exit(0)
}

// from :  https://github.com/a-h/round
//
func round(v float64, decimals int) float64 {
	if math.IsNaN(v) {
		return math.NaN()
	}

	var pow float64 = 1
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	if v < 0 {
		return float64(int((v * pow) - 0.5)) / pow
	}
	return float64(int((v * pow) + 0.5)) / pow
}
