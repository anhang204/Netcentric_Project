package library

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Screen struct {
	app   *tview.Application
	ctx   context.Context
	api   *contract.HTTPClient
	token string
	view  *tview.Flex

	list         *tview.List
	form         *tview.Form
	chapter      *tview.InputField
	status       *tview.DropDown
	feedback     *tview.TextView
	connection   func(string)
	entries      map[string]contract.LibraryEntry
	titles       map[string]string
	ids          []string
	refreshingUI bool
}

func NewScreen(app *tview.Application, ctx context.Context, api *contract.HTTPClient, token string, connection func(string)) *Screen {
	s := &Screen{
		app: app, ctx: ctx, api: api, token: token, connection: connection,
		entries: make(map[string]contract.LibraryEntry), titles: make(map[string]string),
		list: tview.NewList(), feedback: tview.NewTextView().SetWrap(true),
	}
	s.list.SetBorder(true).SetTitle(" My Library ")
	s.list.SetChangedFunc(func(index int, _, _ string, _ rune) { s.selectItem(index) })
	s.chapter = tview.NewInputField().SetLabel("Current chapter").SetFieldWidth(12)
	s.status = tview.NewDropDown().SetLabel("Status").SetOptions([]string{contract.PlanToRead, contract.Reading, contract.Completed}, nil)
	s.status.SetCurrentOption(0)
	s.form = tview.NewForm().AddFormItem(s.chapter).AddFormItem(s.status)
	s.form.AddButton("Save progress", s.save)
	s.form.SetCancelFunc(func() { app.SetFocus(s.list) })
	s.form.SetBorder(true).SetTitle(" Reading Progress ")
	s.list.SetInputCapture(func(key *tcell.EventKey) *tcell.EventKey {
		if s.selectedID() != "" && (key.Key() == tcell.KeyTab || key.Key() == tcell.KeyEnter) {
			s.selectItem(s.list.GetCurrentItem())
			app.SetFocus(s.form)
			return nil
		}
		return key
	})
	s.view = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().AddItem(s.list, 0, 1, true).AddItem(s.form, 0, 1, false), 0, 1, true).
		AddItem(s.feedback, 3, 0, false)
	s.feedback.SetText("Select a manga and press Enter to edit. Esc returns to the list.")
	return s
}

func (s *Screen) View() tview.Primitive { return s.view }

// Add lets the discovery screen add the selected record and display the outcome.
// Call it from the event loop. done also runs on the event loop.
func (s *Screen) Add(manga contract.Manga, done func(error)) {
	go func() {
		var entry contract.LibraryEntry
		err := s.api.Do(s.ctx, http.MethodPost, "/api/library", s.token, contract.AddMangaRequest{MangaID: manga.ID}, &entry)
		s.queue(func() {
			if err == nil {
				s.titles[entry.MangaID] = manga.Title
				s.accept(entry)
				s.drawList()
				s.feedback.SetText("Added to library.")
			} else {
				s.feedback.SetText(err.Error())
			}
			if done != nil {
				done(err)
			}
		})
	}()
}

func (s *Screen) save() {
	id := s.selectedID()
	if id == "" {
		s.feedback.SetText("Select a manga from your library first.")
		return
	}
	chapter, err := strconv.Atoi(strings.TrimSpace(s.chapter.GetText()))
	if err != nil || chapter < 0 {
		s.feedback.SetText("Enter a whole chapter number of zero or higher.")
		return
	}
	_, status := s.status.GetCurrentOption()
	input := contract.UpdateProgressRequest{CurrentChapter: &chapter, Status: &status}
	s.feedback.SetText("Saving…")
	go func() {
		var entry contract.LibraryEntry
		path := "/api/library/" + url.PathEscape(id) + "/progress"
		err := s.api.Do(s.ctx, http.MethodPut, path, s.token, input, &entry)
		s.queue(func() {
			if err != nil {
				s.feedback.SetText(err.Error())
				return
			}
			if s.accept(entry) {
				s.drawList()
			}
			s.feedback.SetText("Progress saved.")
		})
	}()
}

func (s *Screen) reload() {
	var response contract.LibraryList
	err := s.api.Do(s.ctx, http.MethodGet, "/api/library", s.token, nil, &response)
	s.queue(func() {
		if err != nil {
			s.feedback.SetText(err.Error())
			return
		}
		for _, entry := range response.Items {
			s.accept(entry)
		}
		s.drawList()
		if len(response.Items) == 0 && len(s.entries) == 0 {
			s.feedback.SetText("Your library is empty. Add a manga from discovery.")
		}
	})
}

// accept keeps the newest row when HTTP and TCP results arrive in different orders.
func (s *Screen) accept(entry contract.LibraryEntry) bool {
	if previous, exists := s.entries[entry.MangaID]; exists && previous.Version >= entry.Version {
		return false
	}
	s.entries[entry.MangaID] = entry
	return true
}

func (s *Screen) drawList() {
	selected := s.selectedID()
	s.refreshingUI = true
	s.list.Clear()
	s.ids = s.ids[:0]
	for id := range s.entries {
		s.ids = append(s.ids, id)
	}
	sort.Strings(s.ids)
	active := 0
	for index, id := range s.ids {
		entry := s.entries[id]
		title := s.titles[id]
		if title == "" {
			title = id
		}
		s.list.AddItem(tview.Escape(title), fmt.Sprintf("Chapter %d · %s", entry.CurrentChapter, entry.Status), 0, nil)
		if id == selected {
			active = index
		}
	}
	s.list.SetCurrentItem(active)
	s.refreshingUI = false
	if selected == "" || !s.form.HasFocus() {
		s.selectItem(active)
	}
}

func (s *Screen) selectedID() string {
	index := s.list.GetCurrentItem()
	if index < 0 || index >= len(s.ids) {
		return ""
	}
	return s.ids[index]
}

func (s *Screen) selectItem(index int) {
	if s.refreshingUI || index < 0 || index >= len(s.ids) {
		return
	}
	entry := s.entries[s.ids[index]]
	s.chapter.SetText(strconv.Itoa(entry.CurrentChapter))
	switch entry.Status {
	case contract.Reading:
		s.status.SetCurrentOption(1)
	case contract.Completed:
		s.status.SetCurrentOption(2)
	default:
		s.status.SetCurrentOption(0)
	}
}

func (s *Screen) queue(update func()) {
	if s.ctx.Err() != nil {
		return
	}
	s.app.QueueUpdateDraw(func() {
		if s.ctx.Err() == nil {
			update()
		}
	})
}
