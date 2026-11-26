package bootstrap_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	accountsui "github.com/anhang204/Netcentric_Project/internal/tui/accounts"
	chatui "github.com/anhang204/Netcentric_Project/internal/tui/chat"
	libraryui "github.com/anhang204/Netcentric_Project/internal/tui/library"
	"github.com/gdamore/tcell/v2"
	"github.com/gin-gonic/gin"
	"github.com/rivo/tview"
)

func TestLibraryScreenDisplaysTCPPush(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), testCatalog{})
	f.register("alice")
	origin := f.login("alice")
	f.add(origin)

	ctx, cancel := context.WithCancel(context.Background())
	app := tview.NewApplication().SetScreen(tcell.NewSimulationScreen("UTF-8"))
	connections := make(chan string, 4)
	screen := libraryui.NewScreen(app, ctx, contract.NewHTTPClient(f.baseURL), f.login("alice"), func(status string) {
		if status == "Connected" || status == "Disconnected" {
			connections <- status
		}
	})
	root := screen.View().(*tview.Flex)
	columns := root.GetItem(0).(*tview.Flex)
	list := columns.GetItem(0).(*tview.List)
	form := columns.GetItem(1).(*tview.Form)
	chapter := form.GetFormItem(0).(*tview.InputField)
	app.SetRoot(root, true)
	appDone := make(chan error, 1)
	go func() { appDone <- app.Run() }()
	receiverDone := make(chan struct{})
	go func() {
		defer close(receiverDone)
		screen.RunTCP(f.tcpAddr)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-receiverDone:
		case <-time.After(3 * time.Second):
			t.Error("TCP reader did not exit on logout")
		}
		app.Stop()
		select {
		case err := <-appDone:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("tview did not stop")
		}
	})
	waitForConnection := func(want string) {
		select {
		case status := <-connections:
			if status != want {
				t.Fatalf("expected TCP state %q, got %q", want, status)
			}
		case <-time.After(4 * time.Second):
			t.Fatalf("TUI never reported TCP state %q", want)
		}
	}
	waitForConnection("Connected")
	visibleChapter := func(want string) bool {
		for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
			var visible bool
			app.QueueUpdate(func() {
				if list.GetItemCount() > 0 {
					_, details := list.GetItemText(0)
					visible = strings.Contains(details, want+" ·")
				}
			})
			if visible {
				return true
			}
			time.Sleep(10 * time.Millisecond)
		}
		return false
	}
	if !visibleChapter("Chapter 0") {
		t.Fatal("TUI did not show the initial library")
	}
	f.request(http.MethodPut, "/api/library/one-piece/progress", origin, gin.H{"current_chapter": 12}, 200, nil)
	if !visibleChapter("Chapter 12") {
		t.Fatal("TUI did not display the TCP update")
	}
	var fieldText string
	app.QueueUpdate(func() { fieldText = chapter.GetText() })
	if fieldText != "12" {
		t.Errorf("idle progress form still shows %q after chapter 12 arrived", fieldText)
	}
	app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	var focused bool
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		app.QueueUpdate(func() { focused = form.HasFocus() })
		if focused {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !focused {
		t.Fatal("Enter did not move keyboard focus to the progress form")
	}
	app.QueueUpdate(func() { chapter.SetText("17") })
	f.request(http.MethodPut, "/api/library/one-piece/progress", origin, gin.H{"current_chapter": 13, "status": contract.Reading}, 200, nil)
	if !visibleChapter("Chapter 13") {
		t.Fatal("TCP update did not reach the list during editing")
	}
	app.QueueUpdate(func() { fieldText = chapter.GetText() })
	if fieldText != "17" {
		t.Fatalf("TCP update overwrote the active edit: %q", fieldText)
	}
	pressKeys(app, tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter)
	if !visibleChapter("Chapter 17") {
		t.Fatal("saving progress from the keyboard did not update the row")
	}
	var saved contract.LibraryList
	f.request(http.MethodGet, "/api/library", origin, nil, http.StatusOK, &saved)
	if len(saved.Items) != 1 || saved.Items[0].CurrentChapter != 17 {
		t.Fatalf("the TUI chapter was not stored in SQLite: %+v", saved.Items)
	}
	if saved.Items[0].Status != contract.Reading {
		t.Fatalf("saving only the chapter overwrote the other session's status: %+v", saved.Items[0])
	}
	feedback := root.GetItem(1).(*tview.TextView)
	status := form.GetFormItem(1).(*tview.DropDown)
	app.QueueUpdate(func() {
		status.SetCurrentOption(2) // Completed.
		app.SetFocus(form.GetButton(0))
	})
	f.request(http.MethodPut, "/api/library/one-piece/progress", origin, gin.H{"current_chapter": 18}, 200, nil)
	if !visibleChapter("Chapter 18") {
		t.Fatal("TCP update did not reach the list while editing the status")
	}
	pressKeys(app, tcell.KeyEnter)
	waitForText(t, app, feedback, "Progress saved.")
	f.request(http.MethodGet, "/api/library", origin, nil, http.StatusOK, &saved)
	if len(saved.Items) != 1 || saved.Items[0].CurrentChapter != 18 || saved.Items[0].Status != contract.Completed {
		t.Fatalf("saving only the status overwrote the other session's chapter: %+v", saved.Items)
	}
	serverSocket := <-f.connections
	serverSocket.Close()
	waitForConnection("Disconnected")
	f.request(http.MethodPut, "/api/library/one-piece/progress", origin, gin.H{"current_chapter": 19}, 200, nil)
	waitForConnection("Connected")
	if !visibleChapter("Chapter 19") {
		t.Fatal("reconnected TUI did not retrieve the chapter saved during the interruption")
	}
}

func runTerminal(t *testing.T, app *tview.Application, root tview.Primitive, cancel context.CancelFunc) {
	t.Helper()
	app.SetScreen(tcell.NewSimulationScreen("UTF-8")).SetRoot(root, true)
	stopped := make(chan error, 1)
	go func() { stopped <- app.Run() }()
	t.Cleanup(func() {
		cancel()
		app.Stop()
		select {
		case err := <-stopped:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("tview did not stop")
		}
	})
}

func pressKeys(app *tview.Application, keys ...tcell.Key) {
	for _, key := range keys {
		app.QueueEvent(tcell.NewEventKey(key, 0, tcell.ModNone))
	}
}

func typeText(app *tview.Application, text string) {
	for _, character := range text {
		app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, character, tcell.ModNone))
	}
}

func waitForText(t *testing.T, app *tview.Application, view *tview.TextView, want string) {
	t.Helper()
	var got string
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		app.QueueUpdate(func() { got = view.GetText(true) })
		if strings.Contains(got, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("screen showed %q, want text containing %q", got, want)
}

func TestAccountScreenRegistersAndLogsIn(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), nil)
	ctx, cancel := context.WithCancel(context.Background())
	app := tview.NewApplication()
	loggedIn := make(chan string, 1)
	root := accountsui.NewScreen(app, ctx, contract.NewHTTPClient(f.baseURL), func(token string) { loggedIn <- token }).(*tview.Flex)
	runTerminal(t, app, root, cancel)
	feedback := root.GetItem(1).(*tview.TextView)
	typeText(app, "alice")
	pressKeys(app, tcell.KeyTab)
	typeText(app, "demo-password")
	pressKeys(app, tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter)
	waitForText(t, app, feedback, "Account created")
	pressKeys(app, tcell.KeyBacktab, tcell.KeyEnter)
	select {
	case token := <-loggedIn:
		f.request(http.MethodGet, "/api/library", token, nil, http.StatusOK, nil)
	case <-time.After(3 * time.Second):
		t.Fatal("Log in did not supply a JWT to the TUI callback")
	}
}

func TestChatScreenJoinsAndExchangesMessages(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), nil)
	f.register("alice")
	f.register("bob")
	bob := f.ws(f.login("bob"), "general")
	ctx, cancel := context.WithCancel(context.Background())
	app := tview.NewApplication()
	wsURL := "ws" + strings.TrimPrefix(f.baseURL, "http") + "/ws"
	root := chatui.NewScreen(app, ctx, wsURL, f.login("alice"), nil).View().(*tview.Flex)
	runTerminal(t, app, root, cancel)
	transcript := root.GetItem(0).(*tview.TextView)
	feedback := root.GetItem(2).(*tview.TextView)
	pressKeys(app, tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter)
	waitForText(t, app, feedback, "Joined general.")
	pressKeys(app, tcell.KeyBacktab)
	typeText(app, "Hello from the TUI")
	pressKeys(app, tcell.KeyTab, tcell.KeyTab, tcell.KeyEnter)
	if msg := readChat(t, bob); msg.Sender != "alice" || msg.Text != "Hello from the TUI" {
		t.Fatalf("Bob received a different chat message: %+v", msg)
	}
	writeChat(t, bob, contract.ChatMessage{Type: "chat", Room: "general", Text: "Hello Alice"})
	waitForText(t, app, transcript, "bob: Hello Alice")
}

func TestLibraryScreenAddsAndSwitchesAccounts(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), testCatalog{})
	f.register("alice")
	f.register("bob")
	aliceToken, bobToken := f.login("alice"), f.login("bob")
	app := tview.NewApplication()
	api := contract.NewHTTPClient(f.baseURL)
	aliceCtx, cancelAlice := context.WithCancel(context.Background())
	bobCtx, cancelBob := context.WithCancel(context.Background())
	defer cancelAlice()
	connected := make(chan struct{}, 1)
	alice := libraryui.NewScreen(app, aliceCtx, api, aliceToken, func(state string) {
		if state == "Connected" {
			connected <- struct{}{}
		}
	})
	runTerminal(t, app, alice.View(), cancelBob)
	go alice.RunTCP(f.tcpAddr)
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("Alice's TCP connection was not established")
	}
	aliceSocket := <-f.connections
	added := make(chan error, 1)
	app.QueueUpdate(func() {
		alice.Add(contract.Manga{ID: "one-piece", Title: "One Piece"}, func(err error) { added <- err })
	})
	select {
	case err := <-added:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the discovery add callback did not finish")
	}
	aliceView := alice.View().(*tview.Flex)
	waitForText(t, app, aliceView.GetItem(1).(*tview.TextView), "Added to library.")
	cancelAlice()
	bob := libraryui.NewScreen(app, bobCtx, api, bobToken, nil)
	app.QueueUpdateDraw(func() { app.SetRoot(bob.View(), true) })
	go bob.RunTCP(f.tcpAddr)
	bobView := bob.View().(*tview.Flex)
	waitForText(t, app, bobView.GetItem(1).(*tview.TextView), "Your library is empty.")
	f.request(http.MethodPut, "/api/library/one-piece/progress", aliceToken, gin.H{"current_chapter": 3}, 200, nil)
	var bobCount int
	app.QueueUpdate(func() {
		bobCount = bobView.GetItem(0).(*tview.Flex).GetItem(0).(*tview.List).GetItemCount()
	})
	if bobCount != 0 {
		t.Fatal("Alice's library leaked into Bob's TUI")
	}
	err := aliceSocket.SetReadDeadline(time.Now().Add(time.Second))
	if err == nil {
		var one [1]byte
		_, err = aliceSocket.Read(one[:])
	}
	if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
		t.Fatalf("expected Alice's TCP socket to close after logout, got %v", err)
	}
}

func TestConcurrentProgressKeepsIncreasingVersions(t *testing.T) {
	f := start(t, filepath.Join(t.TempDir(), "users.db"), testCatalog{})
	f.register("alice")
	first, second := f.login("alice"), f.login("alice")
	f.add(first)
	api := contract.NewHTTPClient(f.baseURL)
	type result struct {
		entry contract.LibraryEntry
		err   error
	}
	results := make(chan result, 2)
	for i, token := range []string{first, second} {
		go func() {
			var entry contract.LibraryEntry
			err := api.Do(context.Background(), http.MethodPut, "/api/library/one-piece/progress", token, gin.H{"current_chapter": i + 10}, &entry)
			results <- result{entry, err}
		}()
	}
	entries := make(map[int64]contract.LibraryEntry)
	for range 2 {
		r := <-results
		if r.err != nil {
			t.Fatal(r.err)
		}
		entries[r.entry.Version] = r.entry
	}
	if entries[2].MangaID == "" || entries[3].MangaID == "" {
		t.Fatalf("expected version 2 and 3, got %+v", entries)
	}
	var saved contract.LibraryList
	f.request(http.MethodGet, "/api/library", first, nil, 200, &saved)
	if len(saved.Items) != 1 || saved.Items[0] != entries[3] {
		t.Fatalf("latest committed row differs from latest response: %+v", saved)
	}
}
