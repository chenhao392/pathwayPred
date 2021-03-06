package src

import (
	"math"
	"runtime"
	"sync"

	"github.com/gonum/floats"
	"github.com/gonum/matrix/mat64"
)

type fm struct {
	mat64.Matrix
}

type coor struct {
	i [100]int
	j [100]int
}

func (f *coor) SetI(i [100]int) {
	f.i = i
}

func (f coor) I() [100]int {
	return f.i
}

func (f *coor) SetJ(j [100]int) {
	f.j = j
}

func (f coor) J() [100]int {
	return f.j
}

type comKey struct {
	i, j, k, l int
}

//Multiple threads PCC
func ParaCov(data *mat64.Dense, goro int) (covmat *mat64.Dense, err error) {
	nSets, nData := data.Dims()
	if nSets == 0 {
	}
	runtime.GOMAXPROCS(goro)
	c := make([]coor, 1)

	element := coor{}
	var iArr [100]int
	var jArr [100]int
	k := 0

	for i := 0; i < nSets; i++ {
		for j := i; j < nSets; j++ {
			if k <= 99 {
				iArr[k] = i
				jArr[k] = j
			} else {
				element.SetI(iArr)
				element.SetJ(jArr)
				c = append(c, element)
				element = coor{}
				k = 0
				iArr[k] = i
				jArr[k] = j
			}
			k++
		}
	}
	//last coor
	element.SetI(iArr)
	element.SetJ(jArr)
	c = append(c, element)

	//pcc matrix, mean and var sqrt
	covmat = mat64.NewDense(nSets, nSets, nil)
	means := make([]float64, nSets)
	vs := make([]float64, nSets)

	for i := range means {
		means[i] = floats.Sum(data.RawRowView(i)) / float64(nData)
		var element float64
		for j, _ := range data.RawRowView(i) {
			data.Set(i, j, data.At(i, j)-means[i])
			element += data.At(i, j) * data.At(i, j)
		}
		vs[i] = math.Sqrt(element)
	}

	var wg sync.WaitGroup
	in := make(chan coor, goro*40)

	singlePCC := func() {
		for {
			select {
			case element := <-in:

				iArr := element.I()
				jArr := element.J()

				for m := 0; m < len(iArr); m++ {
					i := iArr[m]
					j := jArr[m]
					var cv float64
					for k, val := range data.RawRowView(i) {
						cv += data.At(j, k) * val
					}

					cv = cv / (vs[i] * vs[j])
					if (i == 0 && j == 0 && covmat.At(0, 0) == 0.0) || (i+j) > 0 {
						covmat.Set(i, j, cv)
						covmat.Set(j, i, cv)
					}
				}
				wg.Done()
			}
		}
	}

	wg.Add(len(c))
	for i := 0; i < goro; i++ {
		go singlePCC()
	}
	for i := 0; i < len(c); i++ {
		in <- c[i]
	}
	wg.Wait()

	return covmat, nil
}

//func ParaTransP(PknnExp *mat64.Dense, tY *mat64.Dense, knnIdx map[int][]int, PknnMap map[int]map[int]int, goro int, lamda float64, wg *sync.WaitGroup) (nP *mat64.Dense) {
func ParaTransP(Pknn *mat64.Dense, tY *mat64.Dense, knnIdx map[int][]int, goro int, lamda float64, wg *sync.WaitGroup) (nP *mat64.Dense) {
	nNetworkGene, _ := tY.Caps()
	nP = mat64.NewDense(nNetworkGene, nNetworkGene, nil)
	c := make([]coor, 1)

	element := coor{}
	var iArr [100]int
	var jArr [100]int
	k := 0
	for i := 0; i < nNetworkGene; i++ {
		for j := 0; j < nNetworkGene; j++ {
			if k <= 99 {
				iArr[k] = i
				jArr[k] = j
			} else {
				element.SetI(iArr)
				element.SetJ(jArr)
				c = append(c, element)
				element = coor{}
				k = 0
				iArr[k] = i
				jArr[k] = j
			}
			k++
		}
	}
	//last coor
	element.SetI(iArr)
	element.SetJ(jArr)
	c = append(c, element)
	//the channel as nThreads*40
	in := make(chan coor, goro*40)
	singleTransP := func() {
		for {
			select {
			case element := <-in:
				iArr := element.I()
				jArr := element.J()
				for m := 0; m < len(iArr); m++ {
					i := iArr[m]
					j := jArr[m]
					//each k,l element, kernal part
					ele := 0.0
					for k := 0; k < len(knnIdx[i]); k++ {
						//eleK := Pknn.At(i, kArr[k])
						for l := 0; l < len(knnIdx[j]); l++ {
							//value, _ := knnPairP.Load(comKey{i, j, knnIdx[i][k], knnIdx[j][l]})
							//ele += value.(float64) * tY.At(k, l)
							ele += Pknn.At(i, knnIdx[i][k]) * Pknn.At(j, knnIdx[j][l]) * tY.At(k, l)
							//ele += PknnExp.At(PknnMap[i][k], PknnMap[j][l]) * tY.At(knnIdx[i][k], knnIdx[j][l])
						}
					}
					nP.Set(i, j, ele)

				}
				wg.Done()
			}
		}
	}
	wg.Add(len(c))
	for i := 0; i < goro; i++ {
		go singleTransP()
	}
	for i := 0; i < len(c); i++ {
		in <- c[i]
	}
	wg.Wait()
	for i := 0; i < nNetworkGene; i++ {
		nP.Set(i, i, nP.At(i, i)+lamda)
	}
	return nP
}

func ParaFusKernel(P *mat64.Dense, Y *mat64.Dense, goro int, alpha float64, wg *sync.WaitGroup) (tY *mat64.Dense) {
	nNetworkGene, nOutLabel := Y.Caps()
	tY = mat64.NewDense(nNetworkGene, nNetworkGene, nil)
	c := make([]coor, 1)
	element := coor{}
	var iArr [100]int
	var jArr [100]int
	k := 0
	for i := 0; i < nNetworkGene; i++ {
		for j := 0; j < nNetworkGene; j++ {
			if k <= 99 {
				iArr[k] = i
				jArr[k] = j
			} else {
				element.SetI(iArr)
				element.SetJ(jArr)
				c = append(c, element)
				element = coor{}
				k = 0
				iArr[k] = i
				jArr[k] = j
			}
			k++
		}
	}
	//last coor
	element.SetI(iArr)
	element.SetJ(jArr)
	c = append(c, element)
	//the channel as nThreads*40
	in := make(chan coor, goro*40)

	singleY := func() {
		for {
			select {
			case element := <-in:

				iArr := element.I()
				jArr := element.J()

				for m := 0; m < len(iArr); m++ {
					i := iArr[m]
					j := jArr[m]
					ele := 0.0
					for n := 0; n < nOutLabel; n++ {
						ele += Y.At(i, n) * Y.At(j, n)
					}
					//tY.Set(i, j, P.At(i, j)+alpha*ele)
					tY.Set(i, j, ele)
				}
				wg.Done()
			}
		}
	}
	wg.Add(len(c))
	for i := 0; i < goro; i++ {
		go singleY()
	}
	for i := 0; i < len(c); i++ {
		in <- c[i]
	}
	wg.Wait()
	//tY, _ = dNorm(tY)
	singleKernel := func() {
		for {
			select {
			case element := <-in:

				iArr := element.I()
				jArr := element.J()

				for m := 0; m < len(iArr); m++ {
					i := iArr[m]
					j := jArr[m]
					tY.Set(i, j, P.At(i, j)+alpha*tY.At(i, j))
				}
				wg.Done()
			}
		}
	}
	wg.Add(len(c))
	for i := 0; i < goro; i++ {
		go singleKernel()
	}
	for i := 0; i < len(c); i++ {
		in <- c[i]
	}
	wg.Wait()
	return tY
}

func ParaPairProduct(Pknn *mat64.Dense, knnIdx map[int][]int, PknnMap map[int]map[int]int, goro int, nPknnExp int, wg *sync.WaitGroup) (PknnExp *mat64.Dense) {
	nNetworkGene, _ := Pknn.Caps()
	PknnExp = mat64.NewDense(nPknnExp, nPknnExp, nil)
	c := make([]coor, 1)
	element := coor{}
	var iArr [100]int
	var jArr [100]int
	k := 0
	for i := 0; i < nNetworkGene; i++ {
		for j := 0; j < nNetworkGene; j++ {
			if k <= 99 {
				iArr[k] = i
				jArr[k] = j
			} else {
				element.SetI(iArr)
				element.SetJ(jArr)
				c = append(c, element)
				element = coor{}
				k = 0
				iArr[k] = i
				jArr[k] = j
			}
			k++
		}
	}
	//last coor
	element.SetI(iArr)
	element.SetJ(jArr)
	//the channel as nThreads*40
	in := make(chan coor, 40*goro)
	singleTransP := func() {
		for {
			select {
			case element := <-in:
				iArr := element.I()
				jArr := element.J()
				for m := 0; m < len(iArr); m++ {
					i := iArr[m]
					j := jArr[m]
					ele := 0.0
					for k := 0; k < len(knnIdx[i]); k++ {
						for l := 0; l < len(knnIdx[j]); l++ {
							ele = Pknn.At(i, knnIdx[i][k]) * Pknn.At(j, knnIdx[j][l])
							PknnExp.Set(PknnMap[i][k], PknnMap[j][l], ele)
						}
					}

				}
				wg.Done()
			}
		}
	}
	wg.Add(len(c))
	for i := 0; i < goro; i++ {
		go singleTransP()
	}
	for i := 0; i < len(c); i++ {
		in <- c[i]
	}
	wg.Wait()
	return PknnExp
}
