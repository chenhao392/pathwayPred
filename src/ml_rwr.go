package src

import (
	"fmt"
	"github.com/gonum/matrix/mat64"
	"math"
	"sort"
	"sync"
)

type kv struct {
	Key   int
	Value float64
}

func colNorm(network *mat64.Dense) (normNet *mat64.Dense, n int) {
	n, _ = network.Caps()
	normNet = mat64.NewDense(n, n, nil)
	for j := 0; j < n; j++ {
		s := mat64.Sum(network.ColView(j))
		if s > 0.0 {
			for i := 0; i < n; i++ {
				normNet.Set(i, j, network.At(i, j)/s)
			}
		}
	}
	return normNet, n
}

func dNorm(network *mat64.Dense) (normNet *mat64.Dense, n int) {
	n, _ = network.Caps()
	d := mat64.NewDense(n, n, nil)
	normNet = mat64.NewDense(0, 0, nil)
	for j := 0; j < n; j++ {
		s := math.Sqrt(mat64.Sum(network.RowView(j)))
		if s > 0.0 {
			d.Set(j, j, 1.0/s)
		}
	}
	term1 := mat64.NewDense(0, 0, nil)
	term1.Mul(d, network)
	normNet.Mul(term1, d)
	return normNet, n
}
func PropagateSet(network *mat64.Dense, trYdata *mat64.Dense, idIdx map[string]int, idArr []string, trGeneMap map[string]int, wg *sync.WaitGroup, mutex *sync.Mutex) (sPriorData *mat64.Dense) {
	network, nNetworkGene := dNorm(network)
	//nPriorGene, nPriorLabel := priorData.Caps()
	nTrGene, nTrLabel := trYdata.Caps()
	//ind for prior/label gene set mapping at least one gene to the network
	ind := make([]int, nTrLabel)
	wg.Add(nTrLabel)
	for j := 0; j < nTrLabel; j++ {
		inGene := make([]int, 0)
		for i := 0; i < nTrGene; i++ {
			_, exist := idIdx[idArr[i]]
			_, existTr := trGeneMap[idArr[i]]
			if trYdata.At(i, j) > 0 && exist && existTr {
				ind[j] += 1
				inGene = append(inGene, idIdx[idArr[i]])
			}
		}
		//single
		//aupr better than base line?
		go singleIndFeature(network, nNetworkGene, nTrGene, ind, j, idIdx, idArr, inGene, trGeneMap, trYdata, wg, mutex)
	}
	wg.Wait()

	nOutLabel := 0
	for i := 0; i < nTrLabel; i++ {
		if ind[i] > 1 {
			nOutLabel += 1
		}
	}
	sPriorData = mat64.NewDense(nNetworkGene, nOutLabel, nil)
	c := 0
	wg.Add(nOutLabel)
	for j := 0; j < nTrLabel; j++ {
		if ind[j] > 1 {
			trY := mat64.NewDense(nNetworkGene, 1, nil)
			for i := 0; i < nTrGene; i++ {
				_, exist := idIdx[idArr[i]]
				_, existTr := trGeneMap[idArr[i]]
				if trYdata.At(i, j) > 0 && exist && existTr {
					trY.Set(idIdx[idArr[i]], 0, trYdata.At(i, j))
				}
			}
			prior := mat64.DenseCopyOf(trY)
			go single_sPriorData(network, sPriorData, prior, trY, nNetworkGene, c, wg, mutex)
			//go single_sPriorDataDada(network, sPriorData, prior, nNetworkGene, c)
			c += 1
		}
	}
	wg.Wait()
	return sPriorData
}

func PropagateSetWithPrior(priorData *mat64.Dense, priorGeneID map[string]int, priorIdxToId map[int]string, network *mat64.Dense, trYdata *mat64.Dense, idIdx map[string]int, idxToId map[int]string, idArr []string, trGeneMap map[string]int, wg *sync.WaitGroup, mutex *sync.Mutex) (sPriorData *mat64.Dense) {
	network, nNetworkGene := dNorm(network)
	//nPriorGene, nPriorLabel := priorData.Caps()
	nTrGene, nTrLabel := trYdata.Caps()
	//ind for prior/label gene set mapping at least one gene to the network
	ind := make([]int, nTrLabel)
	wg.Add(nTrLabel)
	for j := 0; j < nTrLabel; j++ {
		inGene := make([]int, 0)
		for i := 0; i < nTrGene; i++ {
			_, exist := idIdx[idArr[i]]
			_, existTr := trGeneMap[idArr[i]]
			if trYdata.At(i, j) > 0 && exist && existTr {
				ind[j] += 1
				inGene = append(inGene, idIdx[idArr[i]])
			}
		}
		//single
		//aupr better than base line?
		go singlePriorFeature(priorData, priorGeneID, priorIdxToId, network, nNetworkGene, nTrGene, ind, j, idIdx, idxToId, idArr, inGene, trGeneMap, trYdata, wg, mutex)
	}
	wg.Wait()

	nOutLabel := 0
	for i := 0; i < nTrLabel; i++ {
		if ind[i] > 1 {
			nOutLabel += 1
		}
	}
	sPriorData = mat64.NewDense(nNetworkGene, nOutLabel, nil)
	c := 0
	wg.Add(nOutLabel)
	for j := 0; j < nTrLabel; j++ {
		if ind[j] > 1 {
			//k passed by ind as (k+1) in singlePriorFeature
			kBest := ind[j] - 1
			trY := mat64.NewDense(nNetworkGene, 1, nil)
			for i := 0; i < nTrGene; i++ {
				_, exist := idIdx[idArr[i]]
				_, existTr := trGeneMap[idArr[i]]
				if trYdata.At(i, j) > 0 && exist && existTr {
					trY.Set(idIdx[idArr[i]], 0, trYdata.At(i, j))
				}
			}
			prior := addPrior(priorData, priorGeneID, priorIdxToId, trY, idIdx, idxToId, kBest, nNetworkGene)
			go single_sPriorData(network, sPriorData, prior, trY, nNetworkGene, c, wg, mutex)
			//go single_sPriorDataDada(network, sPriorData, prior, nNetworkGene, c)
			c += 1
		}
	}
	wg.Wait()
	return sPriorData
}
func single_sPriorData(network *mat64.Dense, sPriorData *mat64.Dense, prior *mat64.Dense, trY *mat64.Dense, nNetworkGene int, c int, wg *sync.WaitGroup, mutex *sync.Mutex) {
	defer wg.Done()
	//n by 1 matrix
	sPrior1 := propagate(network, 0.6, prior)
	mutex.Lock()
	for i := 0; i < nNetworkGene; i++ {
		if trY.At(i, 0) == 1.0 {
			sPriorData.Set(i, c, 1.0)
		} else {
			//value := sPrior1.At(i, 0)
			sPriorData.Set(i, c, sPrior1.At(i, 0))
		}
	}
	mutex.Unlock()
}
func single_sPriorDataDada(network *mat64.Dense, sPriorData *mat64.Dense, prior *mat64.Dense, trY *mat64.Dense, nNetworkGene int, c int, wg sync.WaitGroup, mutex sync.Mutex) {
	defer wg.Done()
	//n by 1 matrix
	sPrior1 := propagate(network, 0.6, prior)
	sPrior2 := propagate(network, 1.0, prior)
	max := 0.0
	for i := 0; i < nNetworkGene; i++ {
		value := sPrior1.At(i, 0) / sPrior2.At(i, 0)
		if !math.IsInf(value, 1) && !math.IsNaN(value) {
			if max < value {
				max = value
			}
		}
	}
	mutex.Lock()
	for i := 0; i < nNetworkGene; i++ {
		value := sPrior1.At(i, 0) / sPrior2.At(i, 0)
		if trY.At(i, 0) == 1.0 {
			sPriorData.Set(i, c, 1.0)
		} else if math.IsInf(value, 1) {
			sPriorData.Set(i, c, 1.0)
		} else if math.IsNaN(value) {
			sPriorData.Set(i, c, 0.0)
		} else {
			sPriorData.Set(i, c, value/max)
		}
	}
	mutex.Unlock()
}

func featureAupr(network *mat64.Dense, prior *mat64.Dense, tsY *mat64.Dense, trY *mat64.Dense) (aupr float64, aupr2 float64) {
	sPrior := propagate(network, 0.6, prior)
	aupr = computeAuprSkipTr(tsY.ColView(0), sPrior.ColView(0), trY.ColView(0))
	aupr2 = ComputeAupr(tsY.ColView(0), sPrior.ColView(0))
	return aupr, aupr2
}

func featureAuprDada(network *mat64.Dense, prior *mat64.Dense, tsY *mat64.Dense, trY *mat64.Dense) (aupr float64) {
	nNetworkGene, _ := network.Caps()
	sPrior1 := propagate(network, 0.6, prior)
	sPrior2 := propagate(network, 1.0, prior)
	sPriorDada := mat64.NewDense(nNetworkGene, 1, nil)
	for i := 0; i < nNetworkGene; i++ {
		value := sPrior1.At(i, 0) / sPrior2.At(i, 0)
		if math.IsInf(value, 1) {
			sPriorDada.Set(i, 0, 1.0)
		} else if math.IsNaN(value) {
			sPriorDada.Set(i, 0, 0.0)
		} else {
			sPriorDada.Set(i, 0, value)
		}
	}
	aupr = computeAuprSkipTr(tsY.ColView(0), sPriorDada.ColView(0), trY.ColView(0))
	return aupr
}

func singleIndFeature(network *mat64.Dense, nNetworkGene int, nPriorGene int, ind []int, j int, idIdx map[string]int, idArr []string, inGene []int, trGeneMap map[string]int, trYdata *mat64.Dense, wg *sync.WaitGroup, mutex *sync.Mutex) {
	defer wg.Done()
	if ind[j] > 1 {
		nCv := 2
		baseLine := float64(ind[j]) / (float64(nCv) * float64(nNetworkGene))
		accumAupr := 0.0
		cvSet := cvSplitNoPerm(len(inGene), 2)
		for i := 0; i < nCv; i++ {
			trY := mat64.NewDense(nNetworkGene, 1, nil)
			tsY := mat64.NewDense(nNetworkGene, 1, nil)
			trGeneCv := make(map[int]int, 0)
			for k := 0; k < len(cvSet[i]); k++ {
				//i:nCV, k:0-k idx,value: idx in network
				trY.Set(inGene[cvSet[i][k]], 0, 1)
				trGeneCv[inGene[cvSet[i][k]]] = 0
			}
			for k := 0; k < nPriorGene; k++ {
				_, exist := idIdx[idArr[k]]
				_, existTr := trGeneMap[idArr[k]]
				if trYdata.At(k, j) > 0 && exist && existTr {
					_, existTrCv := trGeneCv[idIdx[idArr[k]]]
					if !existTrCv {
						tsY.Set(idIdx[idArr[k]], 0, 1)
					}
				}
			}
			prior := mat64.DenseCopyOf(trY)
			featureAupr, _ := featureAupr(network, prior, tsY, trY)
			//BE FIX
			//featureAupr := featureAuprDada(network, prior, tsY, trY)
			accumAupr += featureAupr
			//if featureAupr > baseLine {
			//	fmt.Println(featureAupr, baseLine)
			//}
		}
		accumAupr = accumAupr / float64(nCv)
		mutex.Lock()
		if accumAupr < baseLine {
			ind[j] = 0
			fmt.Println("skip label", j, accumAupr, "<", accumAupr)
		}
		mutex.Unlock()
	}

}

func singlePriorFeature(priorData *mat64.Dense, priorGeneID map[string]int, priorIdxToId map[int]string, network *mat64.Dense, nNetworkGene int, nPriorGene int, ind []int, j int, idIdx map[string]int, idxToId map[int]string, idArr []string, inGene []int, trGeneMap map[string]int, trYdata *mat64.Dense, wg *sync.WaitGroup, mutex *sync.Mutex) {
	defer wg.Done()
	if ind[j] > 1 {
		nCv := 2
		baseLine := float64(ind[j]) / (float64(nCv) * float64(nNetworkGene))
		accumAupr := make(map[int]float64)
		cvSet := cvSplitNoPerm(len(inGene), nCv)
		for i := 0; i < nCv; i++ {
			trY := mat64.NewDense(nNetworkGene, 1, nil)
			tsY := mat64.NewDense(nNetworkGene, 1, nil)
			trGeneCv := make(map[int]int, 0)
			for k := 0; k < len(cvSet[i]); k++ {
				//i:nCV, k:0-k idx,value: idx in network
				trY.Set(inGene[cvSet[i][k]], 0, 1)
				trGeneCv[inGene[cvSet[i][k]]] = 0
			}
			for k := 0; k < nPriorGene; k++ {
				_, exist := idIdx[idArr[k]]
				_, existTr := trGeneMap[idArr[k]]
				if trYdata.At(k, j) > 0 && exist && existTr {
					_, existTrCv := trGeneCv[idIdx[idArr[k]]]
					if !existTrCv {
						tsY.Set(idIdx[idArr[k]], 0, 1)
					}
				}
			}
			for k := 0; k < 100; k += 5 {
				prior := addPrior(priorData, priorGeneID, priorIdxToId, trY, idIdx, idxToId, k, nNetworkGene)
				tmpAupr, _ := featureAupr(network, prior, tsY, trY)
				_, exist := accumAupr[k]
				if !exist {
					accumAupr[k] = tmpAupr
				} else {
					accumAupr[k] += tmpAupr
				}
			}
		}

		//best aupr and corresponding k
		var sortMap []kv
		for k := 0; k < 100; k += 5 {
			sortMap = append(sortMap, kv{k, accumAupr[k]})
		}
		sort.Slice(sortMap, func(i, j int) bool {
			return sortMap[i].Value > sortMap[j].Value
		})
		kBest := sortMap[0].Key
		auprBest := sortMap[0].Value / float64(nCv)
		mutex.Lock()
		if auprBest < baseLine {
			ind[j] = kBest + 1
			fmt.Println("skip label", j, auprBest, "<", baseLine)
		}
		mutex.Unlock()
	}
}

func addPrior(priorData *mat64.Dense, priorGeneId map[string]int, priorIdxToId map[int]string, trY *mat64.Dense, idIdx map[string]int, idxToId map[int]string, k int, n int) (prior *mat64.Dense) {
	inGene := make(map[string]int)
	prior = mat64.NewDense(n, 1, nil)
	for i := 0; i < n; i++ {
		if trY.At(i, 0) == 1.0 {
			//gene_id -> prior index
			_, exist := priorGeneId[idxToId[i]]
			if exist {
				inGene[idxToId[i]] = priorGeneId[idxToId[i]]
				//fmt.Println("inGene:", idxToId[i], priorGeneId[idxToId[i]])
			} //else {
			//fmt.Println("notinGene:", idxToId[i])
			//}
		}
	}
	max := 0.0
	for _, i := range inGene {
		var sortPrior []kv
		for j := 0; j < len(priorGeneId); j++ {
			_, exist := idIdx[priorIdxToId[j]]
			if exist {
				sortPrior = append(sortPrior, kv{j, priorData.At(j, i)})
			}
		}
		sort.Slice(sortPrior, func(a, b int) bool {
			return sortPrior[a].Value > sortPrior[b].Value
		})
		thres := sortPrior[k].Value
		//fmt.Println("thres:", id, i, thres)
		for j := 0; j < len(priorGeneId); j++ {
			if priorData.At(j, i) > thres {
				//fmt.Println("thres in:", id, i, thres)
				_, exist := idIdx[priorIdxToId[j]]
				_, exist2 := inGene[priorIdxToId[j]]
				if exist && !exist2 {
					idx := idIdx[priorIdxToId[j]]
					prior.Set(idx, 0, priorData.At(j, i)+prior.At(idx, 0))
					if prior.At(idx, 0) > max {
						max = prior.At(idx, 0)
					}
				}
			}
		}
	}
	//for i := 0; i < len(priorGeneId); i++ {
	//	_, exist := inGene[priorIdxToId[i]]
	//	if exist {
	//		prior.Set(idIdx[priorIdxToId[i]], 0, 1.0)
	//	} else {
	//		_, exist := idIdx[priorIdxToId[i]]
	//		if exist && prior.At(idIdx[priorIdxToId[i]], 0) > max {
	//			max = prior.At(idIdx[priorIdxToId[i]], 0)
	//			fmt.Println("max as:", max, priorIdxToId[i])
	//		}
	//	} // else {
	//	fmt.Println("other as:", prior.At(idIdx[priorIdxToId[i]], 0), priorIdxToId[i])
	//}
	//}
	//fmt.Println("max:", max)
	for i := 0; i < n; i++ {
		if trY.At(i, 0) == 1.0 {
			prior.Set(i, 0, 1.0)
		} else {
			//_, exist := idIdx[priorIdxToId[i]]
			if prior.At(i, 0) > 0 {
				//fmt.Println("before:", idIdx[priorIdxToId[i]], prior.At(idIdx[priorIdxToId[i]], 0), priorIdxToId[i])
				//fmt.Println("before:", i, priorGeneId[idxToId[i]], prior.At(priorGeneId[idxToId[i]], 0), idxToId[i], trY.At(i, 0))
				prior.Set(i, 0, prior.At(i, 0)/max)
				//fmt.Println("after:", idIdx[priorIdxToId[i]], prior.At(idIdx[priorIdxToId[i]], 0), priorIdxToId[i])
				//fmt.Println("after:", priorGeneId[idxToId[i]], prior.At(priorGeneId[idxToId[i]], 0), idxToId[i])
				//fmt.Println("in Loop:", prior.At(0, 0), priorIdxToId[0])
			}
		}
	}
	//fmt.Println("in Func:", prior.At(0, 0), priorIdxToId[0])
	return prior
	//return trY
}

func propagate(network *mat64.Dense, alpha float64, inPrior *mat64.Dense) (smoothPrior *mat64.Dense) {
	sum := mat64.Sum(inPrior)
	r, _ := inPrior.Caps()
	restart := mat64.NewDense(r, 1, nil)
	prior := mat64.NewDense(r, 1, nil)
	for i := 0; i < r; i++ {
		restart.Set(i, 0, inPrior.At(i, 0)/sum)
		prior.Set(i, 0, inPrior.At(i, 0)/sum)
	}
	thres := 0.0000000001
	maxIter := 1000
	i := 0
	res := 1.0
	for res > thres && i < maxIter {
		prePrior := mat64.DenseCopyOf(prior)
		term1 := mat64.NewDense(0, 0, nil)
		term1.Mul(network, prior)
		for i := 0; i < r; i++ {
			prior.Set(i, 0, alpha*term1.At(i, 0)+(1-alpha)*restart.At(i, 0))
		}
		res = 0.0
		for i := 0; i < r; i++ {
			res += math.Abs(prior.At(i, 0) - prePrior.At(i, 0))
		}
		i += 1
	}
	//var sortMap []kv
	//for i := 0; i < r; i++ {
	//	sortMap = append(sortMap, kv{i, prior.At(i, 0)})
	//}
	//sort.Slice(sortMap, func(i, j int) bool {
	//	return sortMap[i].Value > sortMap[j].Value
	//})

	//thres = sortMap[50].Value
	max := 0.0
	for i := 0; i < r; i++ {
		if prior.At(i, 0) > max {
			max = prior.At(i, 0)
		}
	}

	for i := 0; i < r; i++ {
		//	if prior.At(i, 0) < thres {
		prior.Set(i, 0, prior.At(i, 0)/max)
		//	}
	}
	return prior
}

func FeatureDataStack(sPriorData *mat64.Dense, tsRowName []string, trRowName []string, idIdx map[string]int, tsXdata *mat64.Dense, trXdata *mat64.Dense) (tsXdata1 *mat64.Dense, trXdata1 *mat64.Dense) {
	_, nLabel := sPriorData.Caps()
	tmpTsXdata := mat64.NewDense(len(tsRowName), nLabel, nil)
	tmpTrXdata := mat64.NewDense(len(trRowName), nLabel, nil)
	//tsX
	for k := 0; k < len(tsRowName); k++ {
		for l := 0; l < nLabel; l++ {
			_, exist := idIdx[tsRowName[k]]
			if exist {
				tmpTsXdata.Set(k, l, sPriorData.At(idIdx[tsRowName[k]], l))
			}
		}
	}
	nRow, _ := tsXdata.Caps()
	if nRow == 0 {
		tsXdata = tmpTsXdata
	} else {
		tsXdata = ColStackMatrix(tsXdata, tmpTsXdata)
	}
	//trX
	for k := 0; k < len(trRowName); k++ {
		for l := 0; l < nLabel; l++ {
			_, exist := idIdx[trRowName[k]]
			if exist {
				tmpTrXdata.Set(k, l, sPriorData.At(idIdx[trRowName[k]], l))
			}
		}
	}
	nRow, _ = trXdata.Caps()
	if nRow == 0 {
		trXdata = tmpTrXdata
	} else {
		trXdata = ColStackMatrix(trXdata, tmpTrXdata)
	}
	tsXdata1 = tsXdata
	trXdata1 = trXdata
	return tsXdata1, trXdata1
}