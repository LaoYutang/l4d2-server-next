package controller

import (
	"encoding/binary"
	"fmt"
	"io"
	"l4d2-manager-next/model"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRconReturnsCompleteMultiPacketOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalAudit := enqueueAuditLog
	enqueueAuditLog = func(model.AuditLog) {}
	t.Cleanup(func() { enqueueAuditLog = originalAudit })

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	t.Setenv("L4D2_RCON_URL", listener.Addr().String())
	t.Setenv("L4D2_RCON_PASSWORD", "test-password")
	// The first Chinese character crosses the first 4096-byte packet boundary.
	want := strings.Repeat("x", 4095) + strings.Repeat("插件输出\n", 1000) + "最后一行\n"
	done := make(chan error, 1)
	go func() {
		done <- serveConsoleRCON(listener, want)
	}()

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	form := url.Values{"cmd": {"sm plugins list"}}
	context.Request = httptest.NewRequest(http.MethodPost, "/rcon", strings.NewReader(form.Encode()))
	context.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Rcon(context)
	if err := <-done; err != nil {
		t.Fatalf("mock RCON server: %v", err)
	}
	if response.Code != http.StatusOK || response.Body.String() != want {
		t.Fatalf("RCON HTTP response: status=%d, bytes=%d; want status=200 and %d exact bytes", response.Code, response.Body.Len(), len(want))
	}
}

func serveConsoleRCON(listener net.Listener, output string) error {
	conn, err := listener.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	readRequest := func(wantType int32, wantBody string) (int32, error) {
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
		if header[2] != wantType || string(body) != wantBody+"\x00\x00" {
			return 0, fmt.Errorf("unexpected request type %d, body %q", header[2], body)
		}
		return header[1], nil
	}
	writeResponse := func(id, kind int32, body string) error {
		if err := binary.Write(conn, binary.LittleEndian, [3]int32{int32(len(body) + 10), id, kind}); err != nil {
			return err
		}
		_, err := io.WriteString(conn, body+"\x00\x00")
		return err
	}
	if id, err := readRequest(3, "test-password"); err != nil {
		return err
	} else if err := writeResponse(id, 2, ""); err != nil {
		return err
	}
	commandID, err := readRequest(2, "sm plugins list")
	if err != nil {
		return err
	}
	checkID, err := readRequest(0, "")
	if err != nil {
		return err
	}
	if checkID == commandID {
		return fmt.Errorf("command and check IDs must differ")
	}
	for len(output) > 0 {
		size := min(len(output), 4096)
		if err := writeResponse(commandID, 0, output[:size]); err != nil {
			return err
		}
		output = output[size:]
	}
	return writeResponse(checkID, 0, "")
}
