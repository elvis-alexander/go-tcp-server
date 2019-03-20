package server

import (
	"bytes"
	"fmt"
	"sync"
)

type tile string

const (
	header            = "+---------------+\n"
	EMPTY        tile = "_"
	X            tile = "X"
	O            tile = "O"
	LEN               = 3
	player1Piece      = X
	player2Piece      = O
)

type GameController struct {
	AllGames     map[int64]Game
	gameCtrlLock sync.RWMutex
	currId       int64
}

func NewGameCtrl() GameController {
	return GameController{currId: 0, AllGames: make(map[int64]Game)}
}

// creates new game and return's game id
func (this *GameController) AddGame(player1 Client, player2 Client) Game {
	this.gameCtrlLock.Lock()
	defer this.gameCtrlLock.Unlock()
	defer func() {
		this.currId += 1
	}()
	player1.inGame = true
	player2.inGame = true
	game := NewGame(player1, player2, this.currId)
	this.AllGames[this.currId] = game
	return game
}

func (this *GameController) SubscribeToGame(id int64, client Client) bool {
	this.gameCtrlLock.Lock()
	defer this.gameCtrlLock.Unlock()
	game, ok := this.AllGames[id]
	if !ok {
		return false
	}
	game.addSubscriber(client)
	return true
}

type Game struct {
	player1     Client
	player2     Client
	currPlayer  Client
	board       [][]tile
	endGame     bool
	subscribers []Client
	id          int64
}

func NewGame(player1, player2 Client, id int64) Game {
	board := make([][]tile, LEN)
	for i := 0; i < len(board); i += 1 {
		board[i] = make([]tile, LEN)
	}
	g := Game{player1: player1, player2: player2, board: board, id: id}
	return g
}

func (this Game) addSubscriber(cli Client) {
	this.subscribers = append(this.subscribers, cli)
}

func (this Game) isValidModel(row, col int) bool {
	return !(col < 0 || col >= LEN || row < 0 || row >= LEN || this.board[row][col] != EMPTY)
}

func (this Game) move(row, col int) {
	if this.currPlayer == this.player1 {
		this.board[row][col] = player1Piece
	} else {
		this.board[row][col] = player2Piece
	}
}

func (this Game) swapPlayers(row, col int) {
	if this.currPlayer == this.player1 {
		this.currPlayer = this.player2
	} else {
		this.currPlayer = this.player1
	}
}

func (this Game) publishMove(row, col int) {
	this.declareMessage(this.boardStr(row, col))
}

func (this Game) publishWin() {
	this.declareMessage(fmt.Sprintf("Player=%v has won!\n", this.currPlayer.Username))
}

func (this Game) declareMessage(out string) {
	this.player1.safeWrite(out)
	this.player2.safeWrite(out)
	for _, sub := range this.subscribers {
		sub.safeWrite(out)
	}
}

func (this Game) boardStr(row, col int) string {
	var buff bytes.Buffer
	for _, horiz := range this.board {
		buff.WriteString("\n")
		for _, cell := range horiz {
			buff.WriteString(fmt.Sprintf("|%v", cell))
		}
	}
	buff.WriteString("\n")
	return fmt.Sprintf("Player=%v move to row=%v, col=%v\nboard:%v\n", this.currPlayer.Username, row, col, buff.String())
}

func (this Game) won(row, col int) bool {
	/* check across horizontal, check vertical, check diagonal */
	return this.horizontal(row, col) || this.vertical(row, col) || this.leftDiagonal() || this.rightDiagonal()
}

func (this Game) horizontal(row, col int) bool {
	for c := 0; c < LEN-1; c += 1 {
		if this.board[row][c] == EMPTY || this.board[row][c] != this.board[row][c+1] {
			return false
		}
	}
	return true
}

func (this Game) vertical(row, col int) bool {
	for r := 0; r < LEN-1; r += 1 {
		if this.board[r][col] == EMPTY || this.board[r][col] != this.board[r+1][col] {
			return false
		}
	}
	return true
}

func (this Game) leftDiagonal() bool {
	for i := 0; i < LEN-1; i += 1 {
		if this.board[i][i] == EMPTY || this.board[i][i] != this.board[i+1][i+1] {
			return false
		}
	}
	return true
}

func (this Game) rightDiagonal() bool {
	for i := LEN - 1; i > 0; i -= 1 {
		if this.board[i][LEN-i-1] == EMPTY {
			return false
		}
	}
	return true
}
