package main

import (
	"math"
	"sort"
	"time"
)

const tables = 1_000

type Bot struct {
	transpositionDining [tables]map[uint64]Entry
}

func NewBot() *Bot {
	b := &Bot{}
	for i := range b.transpositionDining {
		b.transpositionDining[i] = map[uint64]Entry{}
	}
	return b
}

// evaluateBoard calculates the board score from the perspective of the red player.
// Red pieces add points, black pieces subtract points. Kings are worth 1.5 points.
func (bot *Bot) evaluateBoard(b *Board) float64 {
	redScore := 0.0
	blackScore := 0.0

	redBonus := 0.0
	blackBonus := 0.0

	pieceCount := 0.0

	blackPieces := []Spot{}
	redPieces := []Spot{}

	furthestRed := Spot{-1, 9}
	furthestBlack := Spot{-1, -1}
	for x := 0; x < boardSize; x++ {
		for y := 0; y < boardSize; y++ {
			piece := b.bitBoard.Get(x, y)
			if piece.exists == 1 {
				bonus := 0.0
				pieceCount++
				if piece.red == 1 {
					redPieces = append(redPieces, Spot{x, y})
					if furthestRed.Y > y {
						furthestRed = Spot{x, y}
					}
					if y == 7 {
						redBonus += 0.5
						if x == 2 || x == 6 {
							redBonus += 0.4
						}
					} else if y == 6 && x == 7 {
						redBonus += 0.9
					}
					if piece.king == 1 {
						redScore += 2.3
					} else {
						redScore += 1
						bonus += []float64{0.2, 0.0, 0.06, 0.08, 0.1, 0.2, 0.4, 0.0}[7-y]
						bonus *= []float64{1.08, 1.04, 1.01, 1.06, 1.05, 1.0, 1.03, 1.07}[x]
					}
				} else {
					blackPieces = append(blackPieces, Spot{x, y})
					if furthestBlack.Y < y {
						furthestBlack = Spot{x, y}
					}
					if y == 0 {
						blackBonus += 0.5
						if x == 1 || x == 5 {
							blackBonus += 0.4
						}
					} else if y == 1 && x == 0 {
						blackBonus += 0.9
					}
					if piece.king == 1 {
						blackScore += 2.3
					} else {
						blackScore += 1
						bonus += []float64{0.2, 0.0, 0.06, 0.08, 0.1, 0.2, 0.4, 0.0}[y]
						bonus *= []float64{1.07, 1.03, 1.0, 1.05, 1.06, 1.01, 1.04, 1.08}[x]
					}
				}
				if piece.red == 1 {
					redScore += bonus
				} else {
					blackScore += bonus
				}
			}
		}
	}
	for _, bc := range blackPieces {
		dif := furthestRed.Y - bc.Y
		if bc.Y < furthestRed.Y && bc.X+dif >= furthestRed.X && furthestRed.X+dif <= bc.X { // the cone of blocking
			redScore += 0.01 / float64(dif)
		}
	}
	for _, rc := range redPieces {
		dif := rc.Y - furthestBlack.Y
		if rc.Y > furthestBlack.Y && rc.X+dif >= furthestBlack.X && furthestBlack.X+dif <= rc.X { // the cone of blocking
			blackScore += 0.01 / float64(dif)
		}
	}
	redScore += redBonus
	blackScore += blackBonus
	return redScore - blackScore
}

type Position struct {
	move  *Move
	value float64
}

type ByValue []Position

func (a ByValue) Len() int           { return len(a) }
func (a ByValue) Less(i, j int) bool { return a[i].value > a[j].value }
func (a ByValue) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

const EXACT = 0
const UPPERBOUND = 1
const LOWERBOUND = 1

type Entry struct {
	pos   Position
	depth float64
	alpha float64
	beta  float64
	typ   int
	ply   int
}

func (bot *Bot) largeHash(b *Board) uint64 {
	out := (b.bitBoard.red + (b.bitBoard.king << 1)) + (b.bitBoard.exists*419 + 1)
	if b.bitBoard.isDoubleJump {
		out += 117.0
	}
	if b.bitBoard.isRedTurn {
		out += 419
		out /= 3
	}
	return out % tables
}

func (bot *Bot) hash(b *Board) uint64 {
	out := (b.bitBoard.king + (b.bitBoard.red << 1)) + (b.bitBoard.exists*143 + 1)
	if b.bitBoard.isDoubleJump {
		out += 147.0
	}
	if b.bitBoard.isRedTurn {
		out += 143
		out /= 5
	}
	return out
}

func (bot *Bot) getPosition(b *Board) (Entry, bool) {
	e, ok := bot.transpositionDining[bot.largeHash(b)][bot.hash(b)]
	return e, ok
}

func (bot *Bot) storePosition(b *Board, e Entry) {
	bot.transpositionDining[bot.largeHash(b)][bot.hash(b)] = e
}

func (bot *Bot) basicSort(b *Board) []Move {
	var positions []Position
	scalar := 1.0
	if !b.bitBoard.isRedTurn {
		scalar = -1.0
	}
	thisPos, okf := bot.getPosition(b)
	for _, move := range b.generateAllMoves() {
		var value float64 = 0
		if okf && thisPos.pos.move != nil && thisPos.pos.move.Equals(move) {
			value = 10_000
			move.isSuperior = true
		}
		b.Save()
		move.MakeMove(b)
		entry, ok := bot.getPosition(b)
		if b.bitBoard.isDoubleJump {
			value += 100
		}
		b.Load()
		if ok {
			value += entry.pos.value * scalar
			// Add the calculated position to the slice
			positions = append(positions, Position{
				value: value * 1_000,
				move:  &move,
			})
			continue
		}
		// Apply sorting logic for 'isMax'
		if b.bitBoard.isRedTurn {
			if move.movedPiece.king != 0 { // If not a king
				value += []float64{0.2, 0.0, 0.06, 0.08, 0.1, 0.2, 0.4, 0.0}[7-move.toY]
				value *= []float64{1.07, 1.03, 1.0, 1.05, 1.06, 1.01, 1.04, 1.08}[move.toX]
				if move.fromY == 7 {
					value -= 1000
				}
				if move.toY == 0 {
					value += 1000
				}
			}
		} else {
			// Apply sorting logic for 'isMin'
			if move.movedPiece.king != 0 { // If not a king
				value += []float64{0.2, 0.0, 0.06, 0.08, 0.1, 0.2, 0.4, 0.0}[move.toY]
				value *= []float64{1.07, 1.03, 1.0, 1.05, 1.06, 1.01, 1.04, 1.08}[move.toX]
				if move.fromY == 0 {
					value -= 1000
				}
				if move.toY == 7 {
					value += 1000
				}
			}
		}

		// Add the calculated position to the slice
		positions = append(positions, Position{
			value: value,
			move:  &move,
		})
	}

	// Sort positions based on the value
	sort.Sort(ByValue(positions))

	// Extract the sorted moves from positions
	var sortedMoves []Move
	for _, pos := range positions {
		sortedMoves = append(sortedMoves, *pos.move)
	}

	return sortedMoves
}

func (bot *Bot) qsearch(b *Board, alpha float64, beta float64, depth float64) float64 {
	b.nodeBudget--
	standPat := bot.evaluateBoard(b)
	if !b.bitBoard.isRedTurn {
		standPat *= -1
	}
	if standPat >= beta || b.nodeBudget <= 0 {
		return beta
	}
	if alpha < standPat {
		alpha = standPat
	}

	allMoves := b.generateAllMoves()
	if len(allMoves) == 0 {
		return float64(-1_000_000 / b.plyCount)
	}

	for _, move := range allMoves {
		checkMove := false
		if move.isJump {
			checkMove = true
		} else if move.toY == 0 && move.movedPiece.red == 1 {
			checkMove = true
		} else if move.toY == 7 && move.movedPiece.red == 0 {
			checkMove = true
		}
		if !checkMove {
			break
		}
		b.Save()
		move.MakeMove(b)
		var score float64
		if b.bitBoard.isDoubleJump {
			score = bot.qsearch(b, alpha, beta, depth+1)
		} else {
			score = -bot.qsearch(b, -beta, -alpha, depth+1)
		}
		b.Load()

		if score >= beta {
			return beta
		}
		if score > alpha {
			alpha = score
		}
	}
	return alpha
}

func (bot *Bot) negascout(b *Board, depth float64, alpha float64, beta float64) Position {
	b.nodeBudget--
	ao := alpha

	entry, ok := bot.getPosition(b)
	if ok && entry.depth >= depth { // Only use entries with at least the current depth
		if entry.typ == EXACT {
			return entry.pos
		}
		if entry.typ == LOWERBOUND {
			alpha = math.Max(alpha, entry.pos.value)
		}
		if entry.typ == UPPERBOUND {
			beta = math.Min(beta, entry.pos.value)
		}
		if alpha >= beta {
			return entry.pos // Prune
		}
	}

	// Base case: if depth is 0 or no moves are available, return the board evaluation
	if depth <= 0 || b.nodeBudget <= 0 {
		return Position{value: bot.qsearch(b, alpha, beta, depth)}
	}

	var allMoves []Move
	if depth >= 4 {
		allMoves = bot.basicSort(b)
	} else {
		allMoves = b.generateAllMoves()
	}

	if len(allMoves) == 0 {
		return Position{value: float64(-1_000_000 / b.plyCount)}
	}

	bestMove := Position{value: -math.MaxFloat64} // Start with very low value (negascout maximizes this)
	for _, move := range allMoves {
		inc := 1.0
		if move.isSuperior {
			inc = 0.9
		}
		b.Save()
		move.MakeMove(b)
		var final Position
		if b.bitBoard.isDoubleJump {
			final = bot.negascout(b, depth-inc, alpha, beta) // Don't flip if was just double jump. < -- IMPORTANT
		} else {
			final = bot.negascout(b, depth-inc, -beta, -alpha) // Initial search
			final.value *= -1
		}
		b.Load()

		if final.value > bestMove.value {
			bestMove.value = final.value
			bestMove.move = &move
		}
		alpha = math.Max(alpha, bestMove.value)

		if b.nodeBudget <= 0 {
			bestMove.move = &move
			return bestMove
		}

		if alpha >= beta {
			break
		}
	}
	tte := Entry{bestMove, depth, alpha, beta, 0, b.plyCount}
	if bestMove.value <= ao {
		tte.typ = UPPERBOUND
	} else if bestMove.value >= beta {
		tte.typ = LOWERBOUND
	} else {
		tte.typ = EXACT
	}
	bot.storePosition(b, tte)
	return bestMove
}

// recursiveDeepening implements the recursive deepening search strategy
func (bot *Bot) recursiveDeepening(b *Board, timeLimit time.Duration) Position {
	if bot.transpositionDining[0] == nil {
		for i := range bot.transpositionDining {
			bot.transpositionDining[i] = map[uint64]Entry{}
		}
	}
	startTime := time.Now()
	var lmm Position
	depth := 9

	for {
		// Check if time limit has passed
		if time.Since(startTime) > timeLimit {
			break
		}

		// Perform the Minimax search with the current depth
		mm := bot.negascout(b, float64(depth), -1_000_000_000, 1_000_000_000)
		mm.value *= -1

		// Update the best move found at this depth
		lmm = mm

		if math.Abs(mm.value) > 1_000 {
			break
		}

		if b.nodeBudget <= 0 {
			break
		}

		// Increase the search depth for the next iteration
		println("Current depth: ", depth)
		depth++
	}
	println("\nTook", (time.Now().UnixMilli() - startTime.UnixMilli()), "ms")

	// Return the best move found within the time limit
	return lmm
}

func (bot *Bot) cleanTrans(b *Board) {
	println("Cleaning transposition table...")
	left := 0
	cleaned := 0
	highest := 0
	for _, tab := range bot.transpositionDining {
		siz := 0
		for key, pos := range tab {
			siz++
			if pos.ply < b.plyCount || pos.ply > b.plyCount+9 {
				delete(tab, key)
				cleaned++
			} else {
				left++
			}
		}
		if siz > highest {
			highest = siz
		}
	}
	println("Clean complete; ", cleaned, "cleaned;", left, "left;", highest, "highest table")
}

// think generates all possible moves, selects one randomly, and executes it.
// This function should be called after every move to make the bot play.
func (bot *Bot) think(b *Board) {
	bot.cleanTrans(b)
	b.nodeBudget = 3_000_000
	// Set a time limit for the bot's thinking process (e.g., 2 seconds)
	timeLimit := time.Second

	// Use recursive deepening to find the best move within the time limit
	mm := bot.recursiveDeepening(b, timeLimit)

	// Make the best move found
	println("Estimated Position at:", int(100*mm.value), "\n")
	mm.move.MakeMove(b)
	b.plyCount++
	if b.bitBoard.isDoubleJump {
		bot.think(b)
	}
}
