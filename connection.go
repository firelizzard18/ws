package main

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"

	"github.com/chzyer/readline"
	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

type session struct {
	ws      *websocket.Conn
	rl      *readline.Instance
	errChan chan error
}

func connect(url, origin string, rlConf *readline.Config) error {
	headers := make(http.Header)
	headers.Add("Origin", origin)

	ws, _, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		return err
	}
	defer ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	rl, err := readline.NewEx(rlConf)
	if err != nil {
		return err
	}
	defer rl.Close()

	sess := &session{
		ws:      ws,
		rl:      rl,
		errChan: make(chan error),
	}

	go sess.readConsole()
	go sess.readWebsocket()

	return <-sess.errChan
}

func (s *session) read() (msg string, err error, ok bool) {
	var line string
	ok = true

	if !options.multiline {
		msg, err = s.rl.Readline()
		goto done
	}

	for err == nil {
		line, err = s.rl.Readline()
		if line == "" {
			break
		}

		msg += "\n" + line
	}

	if msg != "" {
		msg = msg[1:]
	}

done:
	if err == readline.ErrInterrupt {
		err = nil
		ok = false
	}
	return
}

func (s *session) readConsole() {
	for {
		line, err, ok := s.read()
		if err != nil {
			s.errChan <- err
			return
		}
		if !ok {
			continue
		}

		err = s.ws.WriteMessage(websocket.TextMessage, []byte(line))
		if err != nil {
			s.errChan <- err
			return
		}
	}
}

func bytesToFormattedHex(bytes []byte) string {
	text := hex.EncodeToString(bytes)
	return regexp.MustCompile("(..)").ReplaceAllString(text, "$1 ")
}

func (s *session) readWebsocket() {
	defer s.ws.Close()

	rxSprintf := color.New(color.FgGreen).SprintfFunc()

	for {
		msgType, buf, err := s.ws.ReadMessage()
		if err != nil {
			s.errChan <- err
			return
		}

		var text string
		switch msgType {
		case websocket.TextMessage:
			text = string(buf)
		case websocket.BinaryMessage:
			text = bytesToFormattedHex(buf)
		default:
			s.errChan <- fmt.Errorf("unknown websocket frame type: %d", msgType)
			return
		}

		fmt.Fprint(s.rl.Stdout(), rxSprintf("< %s\n", text))
	}
}
