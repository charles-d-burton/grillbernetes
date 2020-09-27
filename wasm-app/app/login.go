package main

import (
	"regexp"
	"strings"

	"github.com/maxence-charriere/go-app/v7/pkg/app"
)

var (
	rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
)

type login struct {
	app.Compo
	email     string
	password  string
	password2 string
	mode      string

	passwordValid bool
	emailValid    bool
}

func (l *login) Render() app.UI {
	div := app.Div().Class("mdl-grid").Body(
		app.Main().Class("mdl-card").Class("md-shadow--6dp").Body(

			app.If(l.mode == "signup",
				app.Div().Class("mdl-card__title mdl-color--primary").Class("mdl-color-text--white").Class("relative").Body(
					app.Button().Class("mdl-button").Class("mdl-button--icon").Body(
						app.I().Class("material-icons").Text("arrow_back"),
					).OnClick(l.OnBackPress),
					app.H2().Class("mdl-card__title-text").Text("K8S Kitchen Signup"),
				),
				app.Div().Class("mdl-card__supporting-text").Body(
					app.Div().Class("mdl-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Class("mdl-textfield__input").ID("login").
							OnChange(l.OnEmailUpdate).
							OnKeyup(l.OnEmailUpdate),
						app.Label().Class("mdl-textfield__label").For("login").Text("Email"),
					),
					app.Div().Class("mdl-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Class("mdl-textfield__input").Type("password").ID("password1").
							OnChange(l.ValidateSignupPassword).
							OnKeyup(l.ValidateSignupPassword),
						app.Label().Class("mdl-textfield__label").For("password1").Text("Password").
							OnChange(l.ValidateSignupPassword).
							OnKeyup(l.ValidateSignupPassword),
					),
					app.Div().Class("mdl-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Class("mdl-textfield__input").Type("password").ID("password2"),
						app.Label().Class("mdl-textfield__label").For("password2").Text("Password Repeat"),
					),
				),
				app.Div().Class("mdl-card__actions").Class("mdl-card--border").Body(
					app.Div().Class("mdl-grid").Body(
						app.Button().Class("mdl-cell").Class("mdl-cell--12-col").Class("mdl-button").Class("mdl-button--raised").
							Class("mdl-button--colored").Class("mdl-color-text--white").Text("Sign up").OnClick(l.OnSignup),
					),
				),
				app.If(l.password != l.password2 || len(l.password) < 12,
					app.Div().Class("mdl-grid").Body(
						app.If(len(l.password) < 12,
							app.Div().Class("mdl-cell").Class("mdl-cell--12-col").Body(
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Passwords needs to be 12 characters"),
							),
						),
						app.If(l.password != l.password2,
							app.Div().Class("mdl-cell").Class("mdl-cell--12-col").Body(
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Passwords do not match"),
							),
						),
					),
				),
			).ElseIf(l.mode == "lostpassword",
				app.Div().Class("mdl-card__title mdl-color--primary").Class("mdl-color-text--white").Class("relative").Body(
					app.Button().Class("mdl-button").Class("mdl-button--icon").Body(
						app.I().Class("material-icons").Text("arrow_back"),
					).OnClick(l.OnBackPress),
					app.H2().Class("mdl-card__title-text").Text("K8S Kitchen Lost Password"),
				),
				app.Div().Class("mdl-card__supporting-text").Body(
					app.Div().Class("mdl-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Class("mdl-textfield__input").Type("email").ID("email"),
						app.Label().Class("mdl-textfield__label").For("email").Text("Email"),
					),
				),
				app.Div().Class("mdl-card__actions").Class("mdl-card--border").Body(
					app.Div().Class("mdl-grid").Body(
						app.Button().Class("mdl-cell").Class("mdl-cell--12-col").Class("mdl-button").Class("mdl-button--raised").
							Class("mdl-button--colored").Class("mdl-color-text--white").Text("Reset Password"),
					),
				),
			).Else(
				app.Div().Class("mdl-card__title mdl-color--primary").Class("mdl-color-text--white").Class("relative").Body(
					app.H2().Class("mdl-card__title-text").Text("K8S Kitchen Login"),
				),

				app.Div().Class("mdl-card__supporting-text").Body(
					app.Div().Class("mdl-textfield").Class("mdl-js-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Type("email").Required(true).Class("mdl-textfield__input").ID("login").Placeholder("Email").
							OnChange(l.OnEmailUpdate).
							OnKeyup(l.OnEmailUpdate),
						app.Label().Class("mdl-textfield__label").For("login"),
					),
					app.Div().Class("mdl-textfield").Class("mdl-js-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Type("password").Required(true).Class("mdl-textfield__input").Placeholder("Password").
							OnChange(l.OnPasswordUpdate).
							OnKeyup(l.OnPasswordUpdate),
						app.Label().Class("mdl-textfield__label").For("password"),
					),
				),

				app.Div().Class("mdl-card__actions").Class("mdl-card--border").Body(
					app.Div().Class("mdl-grid").Body(
						app.Button().Class("mdl-cell").
							Class("mdl-cell--12-col").Class("mdl-button").Class("mdl-button--raised").Class("mdl-button--colored").
							Class("mdl-js-button").Class("mdl-js-ripple-effect").Class("mdl-color-text--white").Text("Login").
							OnClick(l.OnLoginButtonPress),
					),
					app.Div().Class("mdl-grid").Body(
						app.Div().Class("mdl-cell").Class("mdl-cell--12-col").Body(
							app.Div().Class("mdl-color-text--primary").Style("float", "left").Text("Sign up!").OnClick(l.OnSignup),
							app.Div().Class("mdl-color-text--primary").Style("float", "right").Text("Lost Password?").OnClick(l.OnLostPassword),
						),
					),
					app.If(!l.emailValid && len(l.email) != 0,
						app.Div().Class("mdl-grid").Body(
							app.Div().Class("mdl-cell").Class("mdl-cell--12-col").Body(
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Email Invalid"),
							),
						),
					),
					app.If(!l.passwordValid && len(l.password) != 0,
						app.Div().Class("mdl-grid").Body(
							app.Div().Class("mdl-cell").Class("mdl-cell--12-col").Body(
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Password Invalid"),
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Password must be at least 12 characters"),
							),
						),
					),
				),
			),
		),
	)
	return div
}

//TODO: SHould the logic to into a go fun to make it non blocking?  Maybe?
func (l *login) OnEmailUpdate(ctx app.Context, e app.Event) {
	email := ctx.JSSrc.Get("value").String()
	app.Log("Runing key up event: ", email)
	l.email = email
	if len(strings.TrimSpace(email)) == 0 {
		l.emailValid = true //Keep the warning from appearing on empty string
		l.Update()
		return
	}

	if len(email) > 254 || !rxEmail.MatchString(email) {
		l.emailValid = false
		l.Update()
		return
	}
	l.emailValid = true
	l.Update()
}

func (l *login) OnPasswordUpdate(ctx app.Context, e app.Event) {
	app.Log("Updating password")
	l.password = ctx.JSSrc.Get("value").String()
	if len(strings.TrimSpace(l.password)) == 0 {
		l.passwordValid = true //Keep the warning away on empty string
		l.Update()
		return
	}
	if len(strings.TrimSpace(l.password)) < 12 {
		l.passwordValid = false
		l.Update()
		return
	}
	l.passwordValid = true
	l.Update()
}

func (l *login) ValidateSignupPassword(ctx app.Context, e app.Event) {
	password := ctx.JSSrc.Get("value").String()
	l.password = password
	if password != l.password2 {

	}
}

func (l *login) OnSignup(ctx app.Context, e app.Event) {
	app.Log("Signup Pressed")
	if l.passwordValid && l.password == l.password2 {

	}
	l.mode = "signup"
	l.Update()
}

func (l *login) OnLostPassword(ctx app.Context, e app.Event) {
	app.Log("Lost Password Pressed")
	l.mode = "lostpassword"
	l.Update()
}

func (l *login) OnLoginButtonPress(ctx app.Context, e app.Event) {
	if l.passwordValid {
		app.Log(l.email)
		loggedIn.SetTo(true)
		app.Navigate("/")
		l.Update()
		return
	}
}

func (l *login) OnBackPress(ctx app.Context, e app.Event) {
	l.mode = ""
	l.Update()
}
