package main

import (
	"fmt"
	"github.com/fredericlemoine/gotree/io"
	"github.com/fredericlemoine/gotree/io/utils"
)

func main() {
	//fmt.Fprintf(os.Stderr, "Started Quartets\n")
	// quartet, err := utils.ReadRefTree("tests/data/quartets.nw.gz")
	// quartet, err := utils.ReadRefTree("ref_1_31144.nw.gz")
	quartet, err := utils.ReadRefTree("ncbi.nw")
	//nbquartets := 0
	// t := time.Now()
	// c := make(chan os.Signal, 1)
	// signal.Notify(c, os.Interrupt)
	// go func() {
	// 	for sig := range c {
	// 		fmt.Println(sig)
	// 		fmt.Println(time.Now().Sub(t), nbquartets)
	// 	}
	// }()

	if err != nil {
		io.ExitWithMessage(err)
	}
	fmt.Println(quartet.NbTips())
	// quartet.Quartets(true, func(tb1, tb2, tb3, tb4 uint) {
	// 	nbquartets++
	// 	fmt.Fprintf(os.Stderr, "(%d,%d)(%d,%d)\n", tb1, tb2, tb3, tb4)
	// })

	index := quartet.IndexQuartets(false)
	fmt.Printf("Index length: %d\n", len(index.Keys()))
	//fmt.Fprintf(os.Stderr, "End Quartets\n")
	// Total spec quartets ref: 40 842 660 378
	// Total ref              :484 912 516 050

	// Total Spec quartets:     19 291 873 954
	// Total quartets     :  1 473 178 662 276
}
