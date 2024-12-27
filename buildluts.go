package main

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"runtime"
	"sync"
)

// Don't want this tooooo high, as we care mostly about position
const (
	MATECPPER = 900
	MATECP    = 36 * MATECPPER
)

type Pv struct {
	Mate int64
	Cp   int64
	//Line string
}

type Eval struct {
	Pvs []Pv
	//Knodes string
	//Depth  string
}

type Position struct {
	Fen   string
	Evals []Eval
}

func (p *Position) reset() {
	p.Fen = ""
	p.Evals = nil
}

const (
	P_EMPTY    = -1
	P_PAWN_W   = 0
	P_KNIGHT_W = 1
	P_BISHOP_W = 2
	P_ROOK_W   = 3
	P_QUEEN_W  = 4
	P_KING_W   = 5
	P_PAWN_B   = 6
	P_KNIGHT_B = 7
	P_BISHOP_B = 8
	P_ROOK_B   = 9
	P_QUEEN_B  = 10
	P_KING_B   = 11
	P_COUNT    = 12
)

func IsWhite(p int8) bool {
	return p < P_PAWN_B && p > P_EMPTY
}

func PieceName(p int8) string {
	var piece string

	switch p {
	case P_PAWN_W:
		piece = "white pawn"
	case P_KNIGHT_W:
		piece = "white knight"
	case P_BISHOP_W:
		piece = "white bishop"
	case P_ROOK_W:
		piece = "white rook"
	case P_QUEEN_W:
		piece = "white queen"
	case P_KING_W:
		piece = "white king"
	case P_PAWN_B:
		piece = "black pawn"
	case P_KNIGHT_B:
		piece = "black knight"
	case P_BISHOP_B:
		piece = "black bishop"
	case P_ROOK_B:
		piece = "black rook"
	case P_QUEEN_B:
		piece = "black queen"
	case P_KING_B:
		piece = "black king"
	default:
		log.Fatalf("Unknown piece %v", p)
	}
	return piece

}

const (
	MV_UNK = 0
	MV_W   = 1
	MV_B   = 2
)

const (
	IGNORE_MATE     = true
	MIN_ENEMY_COUNT = 1
	MAX_ENEMY_COUNT = 16
	MIN_PIECE_COUNT = 3
	MAX_PIECE_COUNT = 32
	BOARD_W         = 8
	BOARD_H         = BOARD_W
	NUM_SQUARES     = BOARD_W * BOARD_H
)

type BoardSquare struct {
	value int64
	count int64
}

type BoardTable struct {
	board [NUM_SQUARES]BoardSquare
}

type Tables struct {
	eleft_cp_lut  [1 + MAX_ENEMY_COUNT - MIN_ENEMY_COUNT][P_COUNT]BoardTable
	pcount_cp_lut [1 + MAX_PIECE_COUNT - MIN_PIECE_COUNT][P_COUNT]BoardTable

	//TODO more tables?
	count int64
}

func (t *Tables) add(other *Tables) {
	t.count += other.count

	for i := range t.eleft_cp_lut {
		for j := range t.eleft_cp_lut[i] {
			for k := range t.eleft_cp_lut[i][j].board {
				v1 := t.eleft_cp_lut[i][j].board[k].value
				v2 := other.eleft_cp_lut[i][j].board[k].value

				check_overflow(v1, v2)

				t.eleft_cp_lut[i][j].board[k].value = v1 + v2
				t.eleft_cp_lut[i][j].board[k].count += other.eleft_cp_lut[i][j].board[k].count
			}
		}
	}
	for i := range t.pcount_cp_lut {
		for j := range t.pcount_cp_lut[i] {
			for k := range t.pcount_cp_lut[i][j].board {
				v1 := t.pcount_cp_lut[i][j].board[k].value
				v2 := other.pcount_cp_lut[i][j].board[k].value

				check_overflow(v1, v2)

				t.pcount_cp_lut[i][j].board[k].value = v1 + v2
				t.pcount_cp_lut[i][j].board[k].count += other.pcount_cp_lut[i][j].board[k].count
			}
		}
	}
}

type BoardState struct {
	board  [NUM_SQUARES]int8
	move   uint8
	bcount int64
	wcount int64
}

func FromFen(fstr string) *BoardState {
	// read first two fields, ignore the rest unless we need it later
	var out BoardState

	out.move = MV_UNK

	file := 0
	rank := 7
	for i, v := range fstr {
		if v == ' ' || v == '\t' {
			if fstr[i+1] == 'b' {
				out.move = MV_B
			} else if fstr[i+1] == 'w' {
				out.move = MV_W
			} else {
				log.Fatal("Did not find expected turn letter")
			}

			break
		}

		if v <= '9' && v >= '0' {
			newfile := file + int(uint8(v)-uint8('0'))
			for ; file < newfile; file += 1 {
				out.board[(rank*BOARD_W)+file] = P_EMPTY
			}
			continue
		}

		if v == '/' {
			if file != BOARD_W {
				log.Fatalf("FEN Not wide enough?: %q", fstr)
			}
			file = 0
			rank -= 1
			if rank < 0 {
				log.Fatalf("FEN Not tall enough?: %q", fstr)
			}
			continue
		}

		if file >= BOARD_W {
			log.Fatalf("Too wide in %q", fstr)
		}

		var piece int8

		switch v {
		case 'P':
			piece = P_PAWN_W
		case 'N':
			piece = P_KNIGHT_W
		case 'B':
			piece = P_BISHOP_W
		case 'R':
			piece = P_ROOK_W
		case 'Q':
			piece = P_QUEEN_W
		case 'K':
			piece = P_KING_W
		case 'p':
			piece = P_PAWN_B
		case 'n':
			piece = P_KNIGHT_B
		case 'b':
			piece = P_BISHOP_B
		case 'r':
			piece = P_ROOK_B
		case 'q':
			piece = P_QUEEN_B
		case 'k':
			piece = P_KING_B
		default:
			log.Fatalf("Unknown piece %c in %q", v, fstr)
		}

		if IsWhite(piece) {
			out.wcount += 1
		} else {
			out.bcount += 1
		}
		out.board[(rank*BOARD_W)+file] = piece

		file += 1
	}

	if out.bcount < MIN_ENEMY_COUNT || out.bcount > MAX_ENEMY_COUNT || out.wcount < MIN_ENEMY_COUNT || out.wcount > MAX_ENEMY_COUNT || (out.bcount+out.wcount) < MIN_PIECE_COUNT || (out.bcount+out.wcount) > MAX_PIECE_COUNT {
		//log.Fatalf("Bad Piece count in FEN %v %v: %q", out.bcount, out.wcount, fstr)
		// happens, just ignore these ones
		return nil
	}

	if out.move == MV_UNK {
		log.Fatalf("Didn't get move from fen: %q", fstr)
	}

	return &out
}

func check_overflow(a, b int64) {
	if b < 0 && (a+b) > a {
		log.Panicf("Underflow: %v %v", a, b)
	}

	if b > 0 && (a+b) < a {
		log.Panicf("Overflow: %v %v", a, b)
	}
}

type AvgTable struct {
	Piece          string
	Condition      string
	ConditionValue int32
	Board          [NUM_SQUARES]int32
	NumCases       int32
}

func collect(ch chan *Tables, wg *sync.WaitGroup) {
	var final Tables
	scount := 0

	for in_tbl := range ch {
		scount += 1

		final.add(in_tbl)
	}

	log.Printf("Collected %v slices, with %v games", scount, final.count)

	// make the out tables
	out := make([]AvgTable, 0, (len(final.eleft_cp_lut)*len(final.eleft_cp_lut[0]))+(len(final.pcount_cp_lut)*len(final.pcount_cp_lut[0])))

	for i := range final.eleft_cp_lut {
		for j := range final.eleft_cp_lut[i] {
			outb := AvgTable{}
			outb.Piece = PieceName(int8(j))
			outb.Condition = "Number of Enemies"
			outb.ConditionValue = int32(i + MIN_ENEMY_COUNT)

			var cases int32 = 0

			for k := range final.eleft_cp_lut[i][j].board {
				v := final.eleft_cp_lut[i][j].board[k].value
				c := final.eleft_cp_lut[i][j].board[k].count

				var avg int64 = 0

				if c != 0 {
					avg = v / c
					if avg > math.MaxInt32 || avg < math.MinInt32 {
						log.Fatalf("Oops, can't be held by an int32? %v / %v = %v", v, c, avg)
					}
				}

				outb.Board[k] = int32(avg)

				cases += int32(c)
			}

			outb.NumCases = cases
			out = append(out, outb)
		}
	}
	for i := range final.pcount_cp_lut {
		for j := range final.pcount_cp_lut[i] {
			outb := AvgTable{}
			outb.Piece = PieceName(int8(j))
			outb.Condition = "Number of pieces"
			outb.ConditionValue = int32(i + MIN_PIECE_COUNT)

			var cases int32 = 0

			for k := range final.pcount_cp_lut[i][j].board {
				v := final.pcount_cp_lut[i][j].board[k].value
				c := final.pcount_cp_lut[i][j].board[k].count

				var avg int64 = 0

				if c != 0 {
					avg = v / c
					if avg > math.MaxInt32 || avg < math.MinInt32 {
						log.Fatalf("Oops, can't be held by an int32? %v / %v = %v", v, c, avg)
					}
				}

				outb.Board[k] = int32(avg)

				cases += int32(c)
			}

			outb.NumCases = cases
			out = append(out, outb)
		}
	}

	// write out the boards as json
	fname := "./boards.json"
	f, err := os.Create(fname)
	if err != nil {
		log.Fatalf("Error creating output json file: %v", err)
	}

	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.Encode(out)

	log.Printf("Wrote to %q", fname)

	// make graphs too
	//TODO

	wg.Done()
}

func process(f *os.File, start int64, end int64, wg *sync.WaitGroup, ch chan *Tables) {
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

	tbls.count = 0
	for {
		// check if we are at the end
		at, err := f.Seek(0, 1)
		if err != nil {
			log.Fatalf("Checking Position: %v", err)
		}

		if at >= end {
			break
		}

		p.reset()
		err = decoder.Decode(&p)
		if err != nil {
			log.Fatalf("Decoding err: %v", err)
		}

		bs := FromFen(p.Fen)
		if bs == nil {
			continue
		}

		// average the lines scores? Gets you a better middle look based on position?
		// should we just use the best eval? Or all of them?

		var acc int64 = 0
		var count int64 = 0
		var best_set bool = false
		var best int64 = 0

		i := 0
		for _, v := range p.Evals[i].Pvs {
			cp := v.Cp
			if v.Mate != 0 {
				if IGNORE_MATE {
					continue
				}
				m := v.Mate
				cp = MATECP
				if m < 0 {
					m = -m
				}

				// TODO maybe just ignore all of these? They are heavy
				cp -= m * MATECPPER
				if cp < 0 {
					//log.Fatalf("Mate too far out! %#v", p)
					// this happens. Just ignore these, I don't wanna deal with it
					continue
				}

				if v.Mate < 0 {
					cp = -cp
				}
			}

			check_overflow(acc, cp)
			acc += cp
			count += 1

			if !best_set || (bs.move == MV_B && cp < best) || (bs.move == MV_W && cp > best) {
				best = cp
			}
		}

		if count == 0 {
			continue
		}

		avg := acc / count

		//avg := best

		for i, v := range bs.board {
			if v == P_EMPTY {
				continue
			}

			ecount := bs.wcount
			if IsWhite(v) {
				ecount = bs.bcount
			}
			ecount -= MIN_ENEMY_COUNT

			old := tbls.eleft_cp_lut[ecount][v].board[i].value
			check_overflow(old, avg)
			tbls.eleft_cp_lut[ecount][v].board[i].value += avg
			tbls.eleft_cp_lut[ecount][v].board[i].count += 1

			pcount := bs.wcount + bs.bcount
			pcount -= MIN_PIECE_COUNT
			old = tbls.pcount_cp_lut[pcount][v].board[i].value
			check_overflow(old, avg)
			tbls.pcount_cp_lut[pcount][v].board[i].value += avg
			tbls.pcount_cp_lut[pcount][v].board[i].count += 1
		}
		tbls.count += 1

		if (tbls.count & 0x1ffff) == 0x10000 {
			log.Printf("@ %v", float64(at-start)/float64(end-start))
		}
	}

	// send the table across the channel
	ch <- &tbls
}

func main() {
	log.Println("Starting...")

	// So we could  stream the decompressed version, and hand out decompressed lines to goroutines
	// first see if it is reasonable to decompress and just hand out file cursors

	var wg sync.WaitGroup
	var collect_wg sync.WaitGroup
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
	ch := make(chan *Tables)
	collect_wg.Add(1)
	go collect(ch, &collect_wg)

	for i := range numworkers {
		wg.Add(1)

		f, err := os.Open(fname)
		if err != nil {
			log.Fatalln(err)
		}
		defer f.Close()

		var start int64 = chunk * i
		var end int64 = start + chunk
		go process(f, start, end, &wg, ch)
	}

	wg.Wait()

	// close the channel to indicate we are done
	close(ch)
	collect_wg.Wait()
}
