package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	boardSize      = 8  // 8x8 grid
	squareSize     = 50 // Size of each square in pixels
	windowPadding  = 25 // Padding from the window edges to the board
	boardPixelSize = squareSize * boardSize
)

type Spot struct {
	X, Y int
}

// Square represents a single square in the grid
type Square struct {
	X, Y     int
	Clicked  bool
	Occupied bool
	Color    color.Color
}

// Board represents the 8x8 grid of squares
type Board struct {
	bitBoard      *BitBoard
	squares       [boardSize][boardSize]Square
	mouseDown     bool
	selectedPiece *Square
	possibleMoves []Move
	bbStack       []*BitBoard
	plyCount      int
	nodeBudget    int
}

type BitBoard struct {
	exists       uint64
	red          uint64
	king         uint64
	isRedTurn    bool
	isDoubleJump bool
	djx          int
	djy          int
}

type BBResult struct {
	exists uint64
	red    uint64
	king   uint64
}

type Move struct {
	fromX, fromY int
	toX, toY     int
	isJump       bool
	movedPiece   BBResult
	isSuperior   bool
}

var bot1 = NewBot()
var bot2 = NewMBot()

func (b *Board) Save() {
	b.bbStack = append(b.bbStack, &BitBoard{
		b.bitBoard.exists,
		b.bitBoard.red,
		b.bitBoard.king,
		b.bitBoard.isRedTurn,
		b.bitBoard.isDoubleJump,
		b.bitBoard.djx,
		b.bitBoard.djy,
	})
	b.plyCount++
}

// Load pops the last BitBoard state from the stack and restores it as the current state.
func (b *Board) Load() {
	// Ensure there is a state to load
	if len(b.bbStack) == 0 {
		return
	}
	// Pop the last state
	lastIndex := len(b.bbStack) - 1
	b.bitBoard = b.bbStack[lastIndex]
	b.bbStack = b.bbStack[:lastIndex] // Remove the last element
	b.plyCount--
}

func (m *Move) Equals(m2 Move) bool {
	return m.fromX == m2.fromX &&
		m.fromY == m2.fromY &&
		m.toX == m2.toX &&
		m.toY == m2.toY
}

func (m *Move) MakeMove(b *Board) {
	b.bitBoard.isDoubleJump = false
	b.bitBoard.Clear(m.fromX, m.fromY)
	b.bitBoard.Set(m.toX, m.toY, 1, m.movedPiece.red, m.movedPiece.king)
	if m.isJump {
		cx := int((float64(m.toX) + float64(m.fromX)) / 2)
		cy := int((float64(m.toY) + float64(m.fromY)) / 2)
		b.bitBoard.Clear(cx, cy)

		if b.hasJump(m.toX, m.toY) {
			b.bitBoard.isRedTurn = !b.bitBoard.isRedTurn
			b.bitBoard.isDoubleJump = true
			b.bitBoard.djx = m.toX
			b.bitBoard.djy = m.toY
		}
	}
	if b.bitBoard.isRedTurn && m.toY == 0 {
		b.bitBoard.Set(m.toX, m.toY, 1, m.movedPiece.red, 1)
	} else if !b.bitBoard.isRedTurn && m.toY == 7 {
		b.bitBoard.Set(m.toX, m.toY, 1, m.movedPiece.red, 1)
	}
	b.bitBoard.isRedTurn = !b.bitBoard.isRedTurn
	b.selectedPiece = nil
	b.possibleMoves = nil
}

func (bb *BitBoard) Set(x int, y int, exists uint64, red uint64, king uint64) {
	bb.Clear(x, y)
	shifter := uint64(1) << (x + y*8)
	bb.exists |= exists * shifter
	bb.red |= red * shifter
	bb.king |= king * shifter
}

func (bb *BitBoard) Clear(x int, y int) {
	shifter := uint64(1) << (x + y*8)
	bb.exists &^= shifter
	bb.red &^= shifter
	bb.king &^= shifter
}

func (bb *BitBoard) Get(x int, y int) BBResult {
	shifter := uint64(1) << (x + y*8)
	out := BBResult{}
	if (bb.exists & shifter) != 0 {
		out.exists = 1
	}
	if (bb.red & shifter) != 0 {
		out.red = 1
	}
	if (bb.king & shifter) != 0 {
		out.king = 1
	}
	return out
}

// NewBoard initializes a new board with checkered pattern and pieces in starting positions
func NewBoard() *Board {
	board := &Board{
		bitBoard: &BitBoard{0, 0, 0, true, false, 0, 0},
	}
	for i := 0; i < boardSize; i++ {
		for j := 0; j < boardSize; j++ {
			if (i+j)%2 == 0 {
				board.squares[i][j] = Square{
					X:     i * squareSize,
					Y:     j * squareSize,
					Color: color.RGBA{255, 255, 255, 255},
				}
			} else {
				board.squares[i][j] = Square{
					X:     i * squareSize,
					Y:     j * squareSize,
					Color: color.RGBA{0, 0, 100, 255},
				}
			}

			// Add pieces to the board in starting positions
			if (i+j)%2 != 0 {
				if j < 3 {
					board.bitBoard.Set(i, j, 1, 0, 0)
				} else if j > 4 {
					board.bitBoard.Set(i, j, 1, 1, 0)
				}
			}
		}
	}
	return board
}

func (b *Board) hasJump(x, y int) bool {
	moves := b.moveGenerationAt(x, y)
	for _, move := range moves {
		if move.isJump {
			return true
		}
	}
	return false
}

func (b *Board) moveGenerationAt(x, y int) []Move {
	result := b.bitBoard.Get(x, y)
	var moves []Move
	directions := []struct{ dx, dy int }{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}}

	for _, d := range directions {
		if result.red+result.king < 1 && d.dy < 0 {
			continue
		}
		if (1-result.red)+result.king < 1 && d.dy > 0 {
			continue
		}
		newX, newY := x+d.dx, y+d.dy
		if newX >= 0 && newX < boardSize && newY >= 0 && newY < boardSize {
			dest := b.bitBoard.Get(newX, newY)
			if dest.exists == 0 {
				moves = append(moves, Move{x, y, newX, newY, false, result, false})
			} else if dest.exists == 1 && dest.red != result.red {
				capX, capY := newX+d.dx, newY+d.dy
				if capX >= 0 && capX < boardSize && capY >= 0 && capY < boardSize && b.bitBoard.Get(capX, capY).exists == 0 {
					moves = append(moves, Move{x, y, capX, capY, true, result, false})
				}
			}
		}
	}
	return moves
}

func (b *Board) generateAllMoves() []Move {
	var allMoves []Move
	foundCap := false
	if b.bitBoard.isDoubleJump {
		allMoves = b.moveGenerationAt(b.bitBoard.djx, b.bitBoard.djy)
		var outMoves []Move
		for _, move := range allMoves {
			if move.isJump {
				outMoves = append(outMoves, move)
			}
		}
		return outMoves
	}
	for x := 0; x < boardSize; x++ {
		for y := 0; y < boardSize; y++ {
			result := b.bitBoard.Get(x, y)
			if result.exists == 1 && ((result.red == 1 && b.bitBoard.isRedTurn) || (result.red == 0 && !b.bitBoard.isRedTurn)) {
				moves := b.moveGenerationAt(x, y)
				if !foundCap {
				capSearch:
					for _, move := range moves {
						if move.isJump {
							foundCap = true
							break capSearch
						}
					}
				}
				allMoves = append(allMoves, moves...)
			}
		}
	}
	var outMoves []Move
	if foundCap {
		for _, move := range allMoves {
			if move.isJump {
				outMoves = append(outMoves, move)
			}
		}
	} else {
		outMoves = allMoves
	}
	return outMoves
}

func (b *Board) Update() {
	allMovs := b.generateAllMoves()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !b.mouseDown {
			mouseX, mouseY := ebiten.CursorPosition()
			x, y := mouseX/squareSize, mouseY/squareSize
			if x >= 0 && x < boardSize && y >= 0 && y < boardSize {
				if b.selectedPiece == nil {
					result := b.bitBoard.Get(x, y)
					if result.exists == 1 && ((result.red == 1 && b.bitBoard.isRedTurn) || (result.red == 0 && !b.bitBoard.isRedTurn)) {
						b.selectedPiece = &b.squares[x][y]
						posMovs := []Move{}
						for _, move := range allMovs {
							if move.fromX == x && move.fromY == y {
								posMovs = append(posMovs, move)
							}
						}
						b.possibleMoves = posMovs
					}
				} else {
					for _, move := range b.possibleMoves {
						if move.toX == x && move.toY == y {
							move.MakeMove(b)
							b.plyCount++
							if !b.bitBoard.isDoubleJump {
								bot1.think(b) // Call the bot's think function after each move
							}
							break
						}
					}
					b.selectedPiece = nil
					b.possibleMoves = nil
				}
			}
		}
		b.mouseDown = true
	} else {
		b.mouseDown = false
	}
}

func (b *Board) Draw(screen *ebiten.Image) {
	for i := 0; i < boardSize; i++ {
		for j := 0; j < boardSize; j++ {
			square := b.squares[i][j]
			bbS := b.bitBoard.Get(i, j)

			ebitenutil.DrawRect(screen, float64(square.X), float64(square.Y), squareSize, squareSize, square.Color)

			if bbS.exists != 0 {
				var pieceColor = color.RGBA{0, 0, 0, 255}
				if bbS.red != 0 {
					pieceColor = color.RGBA{255, 0, 0, 255}
				}
				if bbS.king != 0 {
					ebitenutil.DrawCircle(screen, float64(square.X+squareSize/2), float64(square.Y+squareSize/2), squareSize/3+3, color.White)
				}
				ebitenutil.DrawCircle(screen, float64(square.X+squareSize/2), float64(square.Y+squareSize/2), squareSize/3, pieceColor)
			}
		}
	}

	for _, move := range b.possibleMoves {
		highlightColor := color.RGBA{0, 255, 0, 128}
		ebitenutil.DrawRect(screen, float64(move.toX*squareSize), float64(move.toY*squareSize), squareSize, squareSize, highlightColor)
	}
}
