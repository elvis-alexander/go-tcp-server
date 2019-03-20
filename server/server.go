package server

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LOGIN = "login"
)

//represents tcp server
type Server struct {
	ln             net.Listener
	timeout        time.Duration
	connectionLock sync.Mutex
	connections    map[string]Client
	ErrCh          chan error // error channel
	gameCtrl       GameController
}

// creates a new server and spawn's 2 thread
// one to listen and one to handle
func NewServerAndListen(address string, timeout time.Duration) (*Server, error) {
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	connections := make(map[string]Client)
	ErrCh := make(chan error)
	gameCtrl := NewGameCtrl()
	serv := Server{ln: ln, timeout: timeout, connections: connections, ErrCh: ErrCh, gameCtrl: gameCtrl}
	go serv.accept()
	return &serv, nil
}

// accepts new connections
func (this *Server) accept() {
	for {
		conn, err := this.ln.Accept()
		if err != nil {
			log.Printf("Unable to accept connection=%v, error=%v", conn.RemoteAddr(), err)
			continue
		}
		go this.handle(conn)
	}
}

// handler for new connections
func (this *Server) handle(conn net.Conn) {
	defer conn.Close()
	log.Printf("Registered connection=%v", conn.RemoteAddr())
	client, err := this.login(conn)
	if err != nil {
		log.Printf("Client connection closed error=%v\n", err)
		return
	}
	defer this.kickOutClient(client)
	if err := this.listenToClient(client); err != nil {
		log.Printf("Connection closed for c=%v err=%v \n", conn.RemoteAddr(), err)
		return
	}
	log.Printf("Game loop closed for c=%v")
}

func (this *Server) listenToClient(client *Client) error {
	inch := make(chan []string)
	errch := make(chan error)
	gameCh := make(chan Game)
	done := make(chan bool)

	go func() {
		for {
			if client.inGame {
				continue
			}
			client.Conn.SetReadDeadline(time.Now().Add(this.timeout))
			bytes, err := client.buffer.ReadBytes('\n')
			if err != nil {
				errch <- fmt.Errorf("client connection closed for client=%v error=%v", client.Conn.RemoteAddr(), err)
				return
			}
			in := strings.TrimSuffix(string(bytes), "\n")
			commands := strings.Split(strings.ToLower(in), " ")
			if len(commands) < 1 {
				client.safeWrite("You were saying: ")
				continue
			}
			inch <- commands
			var some bool = <- done
			fmt.Printf("some: %v", some)
		}
	}()

	go func() {
		for {
			select {
			case game, ok := <-gameCh:
				// if my turn then play otherwise just chill
				fmt.Printf("%v", ok)
				fmt.Printf("serversid1e: %v", game.currPlayer.Username)
				for {
					fmt.Printf("serverside: %v", game.currPlayer.Username)
					if game.currPlayer == *client {

						fmt.Printf("running!!!")
						game.currPlayer.Conn.SetReadDeadline(time.Now().Add(this.timeout))
						bytes, _ := client.buffer.ReadBytes('\n')
						in := strings.TrimSuffix(string(bytes), "\n")
						commands := strings.Split(strings.ToLower(in), " ")
						// PLAY 1 2
						r, _ := strconv.Atoi(commands[1])
						c, _ := strconv.Atoi(commands[2])
						fmt.Printf("val move?: %v  %v %v", game.isValidModel(r, c), r, c)

						if !game.isValidModel(r, c) {
							game.currPlayer.safeWrite("Not a valid move try again!\n")
							continue
						}
						game.move(r, c)
						game.publishMove(r, c)
						if game.won(r, c) {
							game.publishWin()
							done <- true
							break
						}
						game.swapPlayers(r, c)
					}
				}

			}
		}
	}()

	for {
		select {
		case commands, ok := <-inch:
			fmt.Printf("%v", ok)
			switch commands[0] {
			case "whoami":
				client.safeWrite(fmt.Sprintf("%v\n", client.Username))
				break
			case "all_users":
				client.safeWrite(this.allClients())
				break
			case "challenge":
				// spawn off seperate thread to handle game
				opponent := commands[1]
				newGame := this.gameCtrl.AddGame(*client, this.connections[opponent])
				gameCh <- newGame
				break
			case "subscribe":
				gameId, _ := strconv.ParseInt(commands[1], 10, 64)
				this.gameCtrl.SubscribeToGame(gameId, *client)
				client.safeWrite(fmt.Sprintf("Succesfully subscribed to game=%v\n", gameId))
				break
			case "help":
				client.safeWrite(`
	whoami -- returns user name
	all_users -- show's all users online
	challenge <p2-username> -- creates a new game
	all_games -- show's all games
	subscribe <game-id>
`)
				break
			default:
				break
			}
			//done <- true
		}
		fmt.Printf("next select")
	}
	fmt.Printf("exiting for client=%v", client.Username)

	return nil
}

func (this *Server) login(conn net.Conn) (*Client, error) {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	readWriter := bufio.NewReadWriter(reader, writer)

	for {
		// every-time on a new message sent
		conn.SetReadDeadline(time.Now().Add(this.timeout))
		bytes, err := readWriter.ReadBytes('\n')
		if err != nil {
			// send message to caller and continue terminate
			readWriter.WriteString("Bye\n")
			readWriter.Flush()
			return nil, fmt.Errorf("client connection closed for client=%v error=%v", conn.RemoteAddr(), err)
		}
		in := strings.TrimSuffix(string(bytes), "\n")
		fmt.Printf("Recieved message=%v from client=%v\n", in, conn.RemoteAddr())
		tokens := strings.Split(in, " ")
		if len(tokens) < 2 || strings.ToLower(tokens[0]) != LOGIN {
			readWriter.WriteString(fmt.Sprintf("First command must be=%v <username>\n", LOGIN))
			readWriter.Flush()
			continue
		}
		userName := tokens[1]
		client, success := this.addClient(userName, conn, readWriter)
		if !success {
			readWriter.WriteString(fmt.Sprintf("Username '%v' is already used\n", userName))
			readWriter.Flush()
			continue
		}
		client.safeWrite("successfully logged in :)\n")
		return client, nil
	}
}

func (this *Server) allClients() string {
	this.connectionLock.Lock()
	this.connectionLock.Unlock()
	var header string = "+---------------+"
	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("%v\nPlayers\n", header))
	if len(this.connections) > 0 {
		for key := range this.connections {
			b.WriteString(fmt.Sprintf("%v\n", key))
		}
	} else {
		b.WriteString("No users registered")
	}
	b.WriteString(header + "\n")
	return b.String()
}

func (this *Server) addClient(username string, conn net.Conn, buffer *bufio.ReadWriter) (*Client, bool) {
	this.connectionLock.Lock()
	defer this.connectionLock.Unlock()
	if _, ok := this.connections[username]; ok {
		return nil, false
	}
	client := NewClient(username, conn, buffer)
	this.connections[username] = *client
	return client, true
}

func (this *Server) kickOutClient(client *Client) {
	log.Printf("Kicking out user=%v\n", client.Username)
	this.connectionLock.Lock()
	defer this.connectionLock.Unlock()
	delete(this.connections, client.Username)
}

func (this *Server) Close() error {
	err := this.ln.Close()
	if err != nil {
		return err
	}
	for _, client := range this.connections {
		client.close()
	}
	return nil
}
