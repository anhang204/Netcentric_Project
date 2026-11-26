package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/rivo/tview"
)

type Screen struct {
	app        *tview.Application
	ctx        context.Context
	url        string
	token      string
	connection func(string)
	view       *tview.Flex
	transcript *tview.TextView
	feedback   *tview.TextView
	roomInput  *tview.InputField
	textInput  *tview.InputField
	room       string
	cancel     context.CancelFunc
	out        chan contract.ChatMessage
}

func NewScreen(app *tview.Application, ctx context.Context, wsURL, token string, connection func(string)) *Screen {
	s := &Screen{
		app: app, ctx: ctx, url: wsURL, token: token, connection: connection,
		transcript: tview.NewTextView().SetWrap(true).SetWordWrap(true).SetMaxLines(100),
		feedback:   tview.NewTextView().SetWrap(true),
		roomInput:  tview.NewInputField().SetLabel("Room").SetText("general").SetFieldWidth(25),
		textInput:  tview.NewInputField().SetLabel("Message"),
	}
	s.transcript.SetBorder(true).SetTitle(" Discussion ")
	form := tview.NewForm().AddFormItem(s.roomInput).AddFormItem(s.textInput)
	form.AddButton("Join room", s.join).AddButton("Send", s.send)
	s.feedback.SetText("Choose a room and select Join room.")
	s.view = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(s.transcript, 0, 1, false).
		AddItem(form, 7, 0, true).
		AddItem(s.feedback, 2, 0, false)
	return s
}

func (s *Screen) View() tview.Primitive { return s.view }

func (s *Screen) join() {
	room := strings.TrimSpace(s.roomInput.GetText())
	if room == "" || len(room) > 32 {
		s.feedback.SetText("Enter a room name of 1–32 bytes.")
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel
	s.room = ""
	s.out = make(chan contract.ChatMessage, 8)
	s.out <- contract.ChatMessage{Type: "join", Room: room}
	s.feedback.SetText("Joining room…")
	if s.connection != nil {
		s.connection("Connecting")
	}
	go s.run(ctx, cancel, s.out)
}

func (s *Screen) send() {
	if s.room == "" {
		s.feedback.SetText("Join a room before sending a message.")
		return
	}
	text := strings.TrimSpace(s.textInput.GetText())
	if text == "" || len(text) > 500 {
		s.feedback.SetText("Enter a message of 1–500 bytes.")
		return
	}
	select {
	case s.out <- contract.ChatMessage{Type: "chat", Room: s.room, Text: text}:
		s.textInput.SetText("")
	default:
		s.feedback.SetText("Chat is busy. Try sending again.")
	}
}

func (s *Screen) show(msg contract.ChatMessage) {
	switch msg.Type {
	case "joined":
		s.room = msg.Room
		s.transcript.Clear()
		s.feedback.SetText("Joined " + s.room + ".")
		if s.connection != nil {
			s.connection("Connected")
		}
	case "chat":
		if msg.Room == s.room {
			fmt.Fprintf(s.transcript, "%s: %s\n", msg.Sender, msg.Text)
			s.transcript.ScrollToEnd()
		}
	case "error":
		s.feedback.SetText(msg.Text)
	}
}

func (s *Screen) queue(ctx context.Context, update func()) {
	if ctx.Err() != nil {
		return
	}
	s.app.QueueUpdateDraw(func() {
		if ctx.Err() == nil {
			update()
		}
	})
}
