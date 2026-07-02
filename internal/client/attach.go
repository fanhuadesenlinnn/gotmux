package client

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fanhuadesenlinnn/gotmux/internal/daemon"
	"github.com/fanhuadesenlinnn/gotmux/internal/protocol"
	"github.com/fanhuadesenlinnn/gotmux/internal/terminal"
)

func Attach(socketPath, session string, detachOthers bool) error {
	conn, err := daemon.Dial(socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	width, height := terminal.Size(int(os.Stdout.Fd()))
	pc := protocol.NewConn(conn)
	if err := pc.Write(protocol.Message{
		Type:         protocol.TypeAttach,
		Session:      session,
		Width:        width,
		Height:       height,
		DetachOthers: detachOthers,
	}); err != nil {
		return err
	}
	first, err := pc.Read()
	if err != nil {
		return err
	}
	if first.Type == protocol.TypeError || !first.OK {
		return errors.New(first.Text)
	}

	state, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer func() {
		_ = terminal.Restore(int(os.Stdin.Fd()), state)
		terminal.WriteRestoreScreen()
	}()

	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	defer signal.Stop(resizeCh)
	go func() {
		for range resizeCh {
			w, h := terminal.Size(int(os.Stdout.Fd()))
			_ = pc.Write(protocol.Message{Type: protocol.TypeResize, Width: w, Height: h})
		}
	}()

	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		buf := make([]byte, 8192)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if writeErr := pc.Write(protocol.Message{Type: protocol.TypeInput, Data: append([]byte(nil), buf[:n]...)}); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		msg, err := pc.Read()
		if err != nil {
			return err
		}
		switch msg.Type {
		case protocol.TypeOutput, protocol.TypeStatus:
			if len(msg.Data) > 0 {
				_, _ = os.Stdout.Write(msg.Data)
			}
		case protocol.TypeError:
			if msg.Text != "" {
				_, _ = fmt.Fprintf(os.Stderr, "%s\r\n", msg.Text)
			}
		case protocol.TypeExit:
			return nil
		}
		select {
		case <-inputDone:
			return nil
		default:
		}
	}
}
