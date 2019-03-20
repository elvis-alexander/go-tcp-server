package server

import (
	"bufio"
	"net"
	"time"
)

type Client struct {
	Conn       net.Conn
	Username   string
	TimeJoined time.Time
	buffer     *bufio.ReadWriter
	challenger chan Client
	inGame     bool
}

func NewClient(userName string, conn net.Conn, buffer *bufio.ReadWriter) *Client {
	return &Client{Conn: conn, Username: userName, TimeJoined: time.Now(), buffer: buffer}
}

func (this *Client) safeWrite(str string) {
	this.buffer.WriteString(str)
	this.buffer.Flush()
}

func (this *Client) close() error {
	return this.Conn.Close()
}
