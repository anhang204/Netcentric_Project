package bootstrap_test

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anhang204/Netcentric_Project/internal/contract"
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
	connected := make(chan struct{}, 1)
	screen := libraryui.NewScreen(app, ctx, contract.NewHTTPClient(f.baseURL), f.login("alice"), func(status string) {
		if status == "Connected" {
			connected <- struct{}{}
		}
	})
	root := screen.View().(*tview.Flex)
	columns := root.GetItem(0).(*tview.Flex)
	list := columns.GetItem(0).(*tview.List)
	form := columns.GetItem(1).(*tview.Form)
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
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("TUI never connected to the TCP hub")
	}
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
	app.QueueEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); {
		var focused bool
		app.QueueUpdate(func() { focused = form.HasFocus() })
		if focused {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("Enter did not move keyboard focus to the progress form")
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
