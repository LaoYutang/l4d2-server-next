// Package rcon implements the Source RCON protocol, including multi-packet replies.
package rcon

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	responseValue = 0
	execCommand   = 2
	authResponse  = 2
	authRequest   = 3

	minPacketSize  = 10
	maxPacketSize  = 4096 + minPacketSize
	maxCommandLen  = 1000
	defaultTimeout = 5 * time.Second
)

// Conn is an authenticated Source RCON connection. Execute calls must be sequential.
type Conn struct {
	conn    net.Conn
	nextID  int32
	timeout time.Duration
}

type packet struct {
	id   int32
	kind int32
	body []byte
}

// Dial connects to the server and authenticates with the supplied password.
func Dial(address, password string) (*Conn, error) {
	conn, err := net.DialTimeout("tcp", address, defaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("rcon: connect: %w", err)
	}
	client := &Conn{conn: conn, timeout: defaultTimeout}
	if err := client.authenticate(password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) requestID() int32 {
	c.nextID++
	if c.nextID <= 0 {
		c.nextID = 1
	}
	return c.nextID
}

func (c *Conn) authenticate(password string) error {
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return fmt.Errorf("rcon: set authentication deadline: %w", err)
	}
	id := c.requestID()
	if err := c.writePacket(id, authRequest, password); err != nil {
		return err
	}
	for {
		response, err := c.readPacket()
		if err != nil {
			return err
		}
		// Source normally sends an empty RESPONSE_VALUE before AUTH_RESPONSE.
		if response.kind == responseValue && response.id == id {
			continue
		}
		if response.kind != authResponse {
			return fmt.Errorf("rcon: invalid authentication response type %d", response.kind)
		}
		if response.id == -1 {
			return fmt.Errorf("rcon: authentication failed")
		}
		if response.id != id {
			return fmt.Errorf("rcon: authentication response for another request")
		}
		return nil
	}
}

// Execute returns all response packets for a command, preserving their bytes and order.
// An empty RESPONSE_VALUE request with its own ID marks the end of the response,
// as in l4d2-with-astrbot's SourceRCON.execute implementation.
func (c *Conn) Execute(command string) (result string, err error) {
	if command == "" {
		return "", fmt.Errorf("rcon: command too small")
	}
	if len(command) > maxCommandLen {
		return "", fmt.Errorf("rcon: command too long")
	}
	// A failed read may leave a partial frame in the stream; do not reuse it.
	defer func() {
		if err != nil {
			_ = c.Close()
		}
	}()
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return "", fmt.Errorf("rcon: set command deadline: %w", err)
	}
	commandID, checkID := c.requestID(), c.requestID()
	if err := c.writePacket(commandID, execCommand, command); err != nil {
		return "", err
	}
	if err := c.writePacket(checkID, responseValue, ""); err != nil {
		return "", err
	}

	var output strings.Builder
	for {
		response, err := c.readPacket()
		if err != nil {
			return "", fmt.Errorf("rcon: incomplete command response: %w", err)
		}
		if response.kind == authResponse && response.id == -1 {
			return "", fmt.Errorf("rcon: authentication failed")
		}
		if response.kind != responseValue {
			continue
		}
		switch response.id {
		case checkID:
			return output.String(), nil
		case commandID:
			// Keep raw bytes intact: a UTF-8 character may span two packets.
			output.Write(response.body)
		}
		// Ignore other IDs, including the previous check's extra terminator packet.
	}
}

func (c *Conn) writePacket(id, kind int32, body string) error {
	size := len(body) + minPacketSize
	if size > maxPacketSize {
		return fmt.Errorf("rcon: request packet too large")
	}
	data := make([]byte, size+4)
	binary.LittleEndian.PutUint32(data[:4], uint32(size))
	binary.LittleEndian.PutUint32(data[4:8], uint32(id))
	binary.LittleEndian.PutUint32(data[8:12], uint32(kind))
	copy(data[12:], body)
	for len(data) > 0 {
		n, err := c.conn.Write(data)
		if err != nil {
			return fmt.Errorf("rcon: write packet: %w", err)
		}
		if n == 0 {
			return fmt.Errorf("rcon: write packet: %w", io.ErrShortWrite)
		}
		data = data[n:]
	}
	return nil
}

func (c *Conn) readPacket() (packet, error) {
	var size int32
	if err := binary.Read(c.conn, binary.LittleEndian, &size); err != nil {
		return packet{}, fmt.Errorf("rcon: read packet size: %w", err)
	}
	if size < minPacketSize || size > maxPacketSize {
		return packet{}, fmt.Errorf("rcon: invalid packet size %d", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return packet{}, fmt.Errorf("rcon: read packet body: %w", err)
	}
	if data[len(data)-2] != 0 || data[len(data)-1] != 0 {
		return packet{}, fmt.Errorf("rcon: invalid packet padding")
	}
	return packet{
		id:   int32(binary.LittleEndian.Uint32(data[:4])),
		kind: int32(binary.LittleEndian.Uint32(data[4:8])),
		body: data[8 : len(data)-2],
	}, nil
}
