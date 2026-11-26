package accounts

import (
	"context"
	"net/http"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/rivo/tview"
)

// NewScreen returns the login form. onLogin runs on the TUI event loop.
func NewScreen(app *tview.Application, ctx context.Context, api *contract.HTTPClient, onLogin func(string)) tview.Primitive {
	username := tview.NewInputField().SetLabel("Username").SetFieldWidth(30)
	password := tview.NewInputField().SetLabel("Password").SetFieldWidth(30).SetMaskCharacter('*')
	feedback := tview.NewTextView().SetWrap(true)
	form := tview.NewForm().AddFormItem(username).AddFormItem(password)
	form.SetBorder(true).SetTitle(" Login / Register ")
	busy := false
	submit := func(path string, login bool) {
		if busy {
			return
		}
		input := contract.Credentials{Username: username.GetText(), Password: password.GetText()}
		busy = true
		feedback.SetText("Contacting backend…")
		go func() {
			var response contract.LoginResponse
			var output any
			if login {
				output = &response
			}
			err := api.Do(ctx, http.MethodPost, path, "", input, output)
			if ctx.Err() != nil {
				return
			}
			app.QueueUpdateDraw(func() {
				if ctx.Err() != nil {
					return
				}
				busy = false
				if err != nil {
					feedback.SetText(err.Error())
					return
				}
				if login {
					password.SetText("")
					onLogin(response.Token)
					return
				}
				feedback.SetText("Account created. Select Log in to continue.")
			})
		}()
	}
	form.AddButton("Log in", func() { submit("/api/auth/login", true) })
	form.AddButton("Register", func() { submit("/api/auth/register", false) })
	return tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(form, 9, 0, true).
		AddItem(feedback, 3, 0, false)
}
