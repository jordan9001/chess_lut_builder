package main

import (
	"encoding/json"
	"log"
	"os"
	"runtime"
	"sync"
)

const (
	MATECPPER = 1000
	MATECP    = 36 * MATECPPER
)

type Pv struct {
	Mate int64
	Cp   int64
	Line string
}

type Eval struct {
	Pvs    []Pv
	Knodes string
	Depth  string
}

type Position struct {
	Fen   string
	Evals []Eval
}

const (
	P_PAWN_W = 0
	P_KNIGHT_W = 1
	P_BISHOP_W = 2
	P_ROOK_W = 3
	P_QUEEN_W = 4
	P_KING_W = 5
	P_PAWN_B = 6
	P_KNIGHT_B = 7
	P_BISHOP_B = 8
	P_ROOK_B = 9
	P_QUEEN_B = 10
	P_KING_B = 11
	P_COUNT = 12
)

const (
	MAX_ENEMY_COUNT = 16
	NUM_SQUARES = 8 * 8
)

type Tables struct {
	eleft_cp_lut [MAX_ENEMY_COUNT][P_COUNT][NUM_SQUARES]int64
	//TODO more tables?
}

func process(f *os.File, start int64, end int64, wg *sync.WaitGroup) {
	defer wg.Done()
	_, err := f.Seek(start, 0)
	if err != nil {
		log.Fatalln(err)
	}

	var b [1]byte
	var p Position

	// unless you are starting at 0, proceed until you get past a newline

	skipped := 0
	if start != 0 {
		for {
			_, err = f.Read(b[:1])
			if err != nil {
				log.Fatalf("Looking for line start: %v", err)
			}

			skipped += 1
			if b[0] == '\n' {
				break
			}
		}
	}

	log.Printf("%v (%v) - %v\n", start, skipped, end)

	decoder := json.NewDecoder(f)

	// create the tables
	var tbls Tables

	count := 0
	for {
		// check if we are at the end
		at, err := f.Seek(0, 1)
		if err != nil {
			log.Fatalf("Checking Position: %v", err)
		}

		if at >= end {
			break
		}

		decoder.Decode(&p)

		// count number of each team

		// for each piece , add to the LuT

		// add to the avg table
		// I don't think we need math/big for this, I think 64 bit is plenty
		// In [1]: 132053332 (num entries) * 6 (guess on num lines) * 30000 (MATECP) = 0x159e4a8cd680
		// still, freak out if the table would overflow

		count += 1

		if (count & 0x1ffff) == 0x10000 {
			log.Printf("@ %v", float64(at-start)/float64(end-start))
		}
	}

}

func main() {
	log.Println("Starting...")

	// So we could  stream the decompressed version, and hand out decompressed lines to goroutines
	// first see if it is reasonable to decompress and just hand out file cursors

	var wg sync.WaitGroup
	fname := "lichess_db_eval.jsonl"

	// get size
	fi, err := os.Stat(fname)
	if err != nil {
		log.Fatalln(err)
	}

	sz := fi.Size()

	// create workers evenly spaced in file
	numworkers := int64(runtime.NumCPU() - 1)
	chunk := sz / numworkers
	log.Printf("Taking %v slices", numworkers)

	// start the collector which will take in the threads
	//TODO

	for i := range numworkers {
		wg.Add(1)

		f, err := os.Open(fname)
		if err != nil {
			log.Fatalln(err)
		}
		defer f.Close()

		var start int64 = chunk * i
		var end int64 = start + chunk
		go process(f, start, end, &wg)
	}

	wg.Wait()
}
