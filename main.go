package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct {
	board *Board
}

// Initialize the game and board
func NewGame() *Game {
	return &Game{
		board: NewBoard(),
	}
}

// Layout specifies the window size
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 400, 400
}

// Update processes input and updates the game state
func (g *Game) Update() error {
	// Pass mouse events to the board
	g.board.Update()
	if ebiten.IsKeyPressed(ebiten.KeyR) && g.board.bitBoard.isRedTurn {
		bot1.think(g.board)
		println("-")
	}
	return nil
}

// Draw renders the game screen, including the board
func (g *Game) Draw(screen *ebiten.Image) {
	// Draw the board
	g.board.Draw(screen)
}

func main() {
	// Create a new game instance
	game := NewGame()

	// Set the window title and start the game
	ebiten.SetWindowSize(400, 400)
	ebiten.SetWindowTitle("Ebiten 8x8 Board")

	// Run the game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
