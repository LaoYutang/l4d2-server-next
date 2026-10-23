package rcon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func startTestServer(t *testing.T, serve func(net.Conn) error) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			done <- err
			return
		}
		done <- serve(conn)
	}()
	t.Cleanup(func() {
		listener.Close()
		if err := <-done; err != nil {
			t.Errorf("mock server: %v", err)
		}
	})
	return listener.Addr().String()
}

// Decode requests independently of the client to check their on-wire format.
func readTestRequest(conn net.Conn, wantType int32, wantBody string) (int32, error) {
	var header [3]int32
	if err := binary.Read(conn, binary.LittleEndian, &header); err != nil {
		return 0, err
	}
	if header[0] < 10 || header[0] > 4106 {
		return 0, fmt.Errorf("invalid request size: %d", header[0])
	}
	body := make([]byte, header[0]-8)
	if _, err := io.ReadFull(conn, body); err != nil {
		return 0, err
	}
	if header[1] <= 0 || header[2] != wantType || string(body) != wantBody+"\x00\x00" {
		return 0, fmt.Errorf("unexpected request: id=%d, type=%d, body=%q", header[1], header[2], body)
	}
	return header[1], nil
}

func testResponse(id, kind int32, body string) []byte {
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, [3]int32{int32(len(body) + 10), id, kind})
	data.WriteString(body)
	data.Write([]byte{0, 0})
	return data.Bytes()
}

func readTestCommand(conn net.Conn, command string) (int32, int32, error) {
	commandID, err := readTestRequest(conn, 2, command)
	if err != nil {
		return 0, 0, err
	}
	checkID, err := readTestRequest(conn, 0, "")
	if err == nil && commandID == checkID {
		err = fmt.Errorf("command and check must use different IDs")
	}
	return commandID, checkID, err
}

func dialTestServer(t *testing.T, serve func(net.Conn) error) *Conn {
	t.Helper()
	address := startTestServer(t, func(conn net.Conn) error {
		id, err := readTestRequest(conn, 3, "test-password")
		if err != nil {
			return err
		}
		if _, err := conn.Write(append(testResponse(id, 0, ""), testResponse(id, 2, "")...)); err != nil {
			return err
		}
		return serve(conn)
	})
	client, err := Dial(address, "test-password")
	if err != nil {
		t.Fatal(err)
	}
	client.timeout = time.Second
	t.Cleanup(func() { client.Close() })
	return client
}

func TestExecuteCollectsCompleteResponse(t *testing.T) {
	utf8Output := strings.Repeat("x", 4095) + "中文输出\n"
	tests := []struct {
		name       string
		chunks     []string
		fragmented bool
		checkBody  string
	}{
		{name: "single packet", chunks: []string{"status\n"}},
		{name: "over 12 KiB", chunks: []string{strings.Repeat("a", 4096), strings.Repeat("b", 4096), strings.Repeat("c", 4096), "end\n"}},
		{name: "short first packet", chunks: []string{"header\n", strings.Repeat("x", 4096), "tail\n"}},
		{name: "exactly 4 KiB", chunks: []string{strings.Repeat("x", 4096)}},
		{name: "empty response"},
		{name: "empty packet before output", chunks: []string{"", "output\n"}},
		{name: "UTF-8 split across packets", chunks: []string{utf8Output[:4096], utf8Output[4096:]}},
		{name: "fragmented TCP stream", chunks: []string{strings.Repeat("a", 4096), "中文\n", "tail"}, fragmented: true},
		{name: "nonempty check response", chunks: []string{"output"}, checkBody: "\x00\x01\x00\x00"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := dialTestServer(t, func(conn net.Conn) error {
				commandID, checkID, err := readTestCommand(conn, "status")
				if err != nil {
					return err
				}
				data := testResponse(commandID+100, 0, "unrelated response")
				data = append(data, testResponse(commandID, 2, "wrong packet type")...)
				for _, body := range test.chunks {
					data = append(data, testResponse(commandID, 0, body)...)
				}
				data = append(data, testResponse(checkID, 0, test.checkBody)...)
				for len(data) > 0 {
					size := len(data)
					if test.fragmented {
						size = min(size, 3)
					}
					n, err := conn.Write(data[:size])
					if err != nil {
						return err
					}
					data = data[n:]
				}
				return nil
			})
			got, err := client.Execute("status")
			want := strings.Join(test.chunks, "")
			if err != nil || got != want {
				t.Fatalf("Execute() returned %d bytes, error %v; want %d exact bytes", len(got), err, len(want))
			}
		})
	}
}

func TestExecuteSequentialCommandsIgnorePreviousTerminator(t *testing.T) {
	responses := []string{strings.Repeat("first\n", 1000), "", "third\n"}
	client := dialTestServer(t, func(conn net.Conn) error {
		seen := make(map[int32]bool)
		for i, response := range responses {
			commandID, checkID, err := readTestCommand(conn, fmt.Sprintf("command%d", i))
			if err != nil {
				return err
			}
			if seen[commandID] || seen[checkID] {
				return fmt.Errorf("request IDs were reused")
			}
			seen[commandID], seen[checkID] = true, true
			var data []byte
			for len(response) > 0 {
				size := min(len(response), 4096)
				data = append(data, testResponse(commandID, 0, response[:size])...)
				response = response[size:]
			}
			data = append(data, testResponse(checkID, 0, "")...)
			// Source can echo the check, then send this additional terminator.
			data = append(data, testResponse(checkID, 0, "\x00\x01\x00\x00")...)
			if _, err := conn.Write(data); err != nil {
				return err
			}
		}
		return nil
	})
	for i, want := range responses {
		got, err := client.Execute(fmt.Sprintf("command%d", i))
		if err != nil || got != want {
			t.Fatalf("command %d: got %d bytes, error %v; want %d exact bytes", i, len(got), err, len(want))
		}
	}
}

func TestDialAuthentication(t *testing.T) {
	tests := []struct {
		name       string
		emptyFirst bool
		responseID int32
		kind       int32
		wantError  string
	}{
		{name: "with empty response", emptyFirst: true, kind: 2},
		{name: "without empty response", kind: 2},
		{name: "wrong password", emptyFirst: true, responseID: -1, kind: 2, wantError: "authentication failed"},
		{name: "wrong ID", responseID: 100, kind: 2, wantError: "another request"},
		{name: "wrong type", kind: 3, wantError: "response type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address := startTestServer(t, func(conn net.Conn) error {
				id, err := readTestRequest(conn, 3, "test-password")
				if err != nil {
					return err
				}
				var data []byte
				if test.emptyFirst {
					data = testResponse(id, 0, "")
				}
				if test.responseID != 0 {
					id = test.responseID
				}
				data = append(data, testResponse(id, test.kind, "")...)
				// Exercise partial headers and padding during authentication too.
				for _, b := range data {
					if _, err := conn.Write([]byte{b}); err != nil {
						return err
					}
				}
				_, err = conn.Read(make([]byte, 1))
				if !errors.Is(err, io.EOF) {
					return fmt.Errorf("client did not close connection: %v", err)
				}
				return nil
			})
			client, err := Dial(address, "test-password")
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) || client != nil {
					if client != nil {
						client.Close()
					}
					t.Fatalf("Dial() = %v, %v; want error containing %q", client, err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				client.Close()
			}
		})
	}
}

func TestExecuteRejectsIncompleteResponse(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantError error
	}{
		{name: "missing end marker", wantError: io.EOF},
		{name: "partial size", data: []byte{10, 0}, wantError: io.ErrUnexpectedEOF},
		{name: "partial body", data: testResponse(2, 0, "more output")[:15], wantError: io.ErrUnexpectedEOF},
		{name: "negative size", data: []byte{255, 255, 255, 255}},
		{name: "undersized packet", data: []byte{9, 0, 0, 0}},
		{name: "oversized packet", data: []byte{255, 255, 255, 127}},
		{name: "invalid padding", data: []byte{10, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := dialTestServer(t, func(conn net.Conn) error {
				id, _, err := readTestCommand(conn, "status")
				if err != nil {
					return err
				}
				data := append(testResponse(id, 0, "partial output"), test.data...)
				_, err = conn.Write(data)
				return err
			})
			got, err := client.Execute("status")
			if got != "" || err == nil || (test.wantError != nil && !errors.Is(err, test.wantError)) {
				t.Fatalf("Execute() = %q, %v; want an error, never partial success", got, err)
			}
			if _, err := client.Execute("status"); !errors.Is(err, net.ErrClosed) {
				t.Fatalf("connection must be closed after a failed read, got %v", err)
			}
		})
	}
}

func TestExecuteTimesOutWithoutEndMarker(t *testing.T) {
	client := dialTestServer(t, func(conn net.Conn) error {
		id, _, err := readTestCommand(conn, "status")
		if err != nil {
			return err
		}
		if _, err := conn.Write(testResponse(id, 0, "partial output")); err != nil {
			return err
		}
		_, err = conn.Read(make([]byte, 1))
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf("client did not close after timeout: %v", err)
		}
		return nil
	})
	client.timeout = 100 * time.Millisecond
	got, err := client.Execute("status")
	var networkError net.Error
	if got != "" || !errors.As(err, &networkError) || !networkError.Timeout() {
		t.Fatalf("Execute() = %q, %v; want timeout without partial success", got, err)
	}
}

func TestExecuteInvalidCommandKeepsConnectionUsable(t *testing.T) {
	client := dialTestServer(t, func(conn net.Conn) error {
		id, checkID, err := readTestCommand(conn, "status")
		if err != nil {
			return err
		}
		_, err = conn.Write(append(testResponse(id, 0, "ok"), testResponse(checkID, 0, "")...))
		return err
	})
	for _, command := range []string{"", strings.Repeat("x", 1001)} {
		if _, err := client.Execute(command); err == nil {
			t.Fatalf("accepted invalid command of length %d", len(command))
		}
	}
	if got, err := client.Execute("status"); err != nil || got != "ok" {
		t.Fatalf("connection unusable after invalid command: %q, %v", got, err)
	}
}
