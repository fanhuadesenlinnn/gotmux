package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
)

func Dial(socketPath string) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, 500*time.Millisecond)
}

func Ensure(socketPath string) error {
	if conn, err := Dial(socketPath); err == nil {
		_ = conn.Close()
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "--server", "-S", socketPath)
	devNull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if devNull != nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	cmd.Env = os.Environ()
	setDetached(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := Dial(socketPath)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server did not start at %s", socketPath)
}

func SendCommand(socketPath string, command []string, session string, width, height int) (protocol.Message, error) {
	conn, err := Dial(socketPath)
	if err != nil {
		return protocol.Message{}, err
	}
	defer conn.Close()

	pc := protocol.NewConn(conn)
	if err := pc.Write(protocol.Message{
		Type:    protocol.TypeCommand,
		Command: command,
		Session: session,
		Width:   width,
		Height:  height,
	}); err != nil {
		return protocol.Message{}, err
	}
	return pc.Read()
}
