package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
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
	otp       string

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
						app.Input().Class("mdl-textfield__input").ID("login").Placeholder("Email").
							OnChange(l.OnEmailUpdate).
							OnKeyup(l.OnEmailUpdate),
						app.Label().Class("mdl-textfield__label").For("login"),
					),
					app.Div().Class("mdl-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Class("mdl-textfield__input").Type("password").ID("password").Placeholder("Password").
							OnChange(l.OnPasswordUpdate).
							OnKeyup(l.OnPasswordUpdate),
						app.Label().Class("mdl-textfield__label").For("password"),
					),
					app.Div().Class("mdl-textfield").Class("mdl-textfield--floating-label").Body(
						app.Input().Class("mdl-textfield__input").Type("password").ID("password2").Placeholder("Password Repeat").
							OnChange(l.ValidateSignupPasswords).
							OnKeyup(l.ValidateSignupPasswords),
						app.Label().Class("mdl-textfield__label").For("password2"),
					),
				),
				app.Div().Class("mdl-card__actions").Class("mdl-card--border").Body(
					app.Div().Class("mdl-grid").Body(
						app.Button().Class("mdl-cell").Class("mdl-cell--12-col").Class("mdl-button").Class("mdl-button--raised").
							Class("mdl-button--colored").Class("mdl-color-text--white").Text("Sign up").OnClick(l.OnSingupRequest),
					),
				),
				app.If(l.password != l.password2 || len(l.password) < 12 || !l.emailValid,
					app.Div().Class("mdl-grid").Body(
						app.Div().Class("mdl-cell").Class("mdl-cell--12-col").Body(
							app.If(!l.emailValid,
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Email not valid"),
							),
							app.If(l.password != l.password2,
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Passwords must match"),
							),
							app.If(len(l.password) < 12,
								app.Div().Class("mdl-color-text--red").Style("float", "center").Text("Password must be at least 12 characters"),
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
							//app.Div().Class("mdl-color-text--primary").Style("float", "center").Text("Enter Code").OnClick(l.OnReqValideteCode),
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

func (l *login) ValidateSignupPasswords(ctx app.Context, e app.Event) {
	l.password2 = ctx.JSSrc.Get("value").String()
	if l.password != l.password2 {
		l.passwordValid = false
		l.Update()
		return
	}
	l.Update()
	l.passwordValid = true
}

func (l *login) OnSignup(ctx app.Context, e app.Event) {
	app.Log("Signup Pressed")
	l.mode = "signup"
	l.Update()
}

func (l *login) OnSingupRequest(ctx app.Context, e app.Event) {
	if l.password == l.password2 && l.emailValid {
		go func() {
			defer l.Update()
			type newUser struct {
				Username string `json:"username"`
				Password string `json:"password"`
				Email    string `json:"email"`
			}

			var user newUser
			user.Username = strings.Split(l.email, "@")[0]
			user.Password = l.password
			user.Email = l.email

			data, err := json.Marshal(&user)
			if err != nil {
				app.Log(err.Error())
			}
			req, _ := http.NewRequest("POST", auth+"register", bytes.NewBuffer(data))
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				app.Log(err.Error())
				l.mode = ""
				l.Update()
			}
			if resp.StatusCode == http.StatusOK {
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					app.Log(err.Error())
				}
				app.Log(string(body))
				app.Dispatch(func() {
					l.mode = "otp"
					l.loginValidating.UnSet()
					l.Update()
				})
				return
			}
		}()
	}
	l.loginValidating.SetTo(true)
	l.Update()
}

func (l *login) OnReqValideteCode(ctx app.Context, e app.Event) {
	l.mode = "otp"
	l.Update()
}

func (l *login) OnUpdateCode(ctx app.Context, e app.Event) {
	app.Log("Updating OTP")
	l.otp = ctx.JSSrc.Get("value").String()
	l.Update()
}

func (l *login) OnValidateCode(ctx app.Context, e app.Event) {
	if l.otp != "" {
		go func() {
			defer l.Update()
			type validation struct {
				OTP      string `json:"otp"`
				Username string `json:"username"`
			}

			var valid validation
			valid.OTP = l.otp
			valid.Username = l.email
			data, err := json.Marshal(&valid)
			if err != nil {
				app.Log(err.Error())
				l.mode = ""
				l.Update()
			}
			app.Log(string(data))
			req, _ := http.NewRequest("POST", auth+"otp", bytes.NewBuffer(data))
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				app.Log(err.Error())
				l.mode = ""
				l.Update()
			}
			if resp.StatusCode == http.StatusOK {
				app.Dispatch(func() {
					l.mode = ""
					l.loginValidating.UnSet()
					l.Update()
				})
				return
			}
			app.Dispatch(func() {
				l.mode = "otp"
				l.loginValidating.UnSet()
				l.Update()
			})
		}()
	}
	l.loginValidating.SetTo(true)
	l.Update()
}

func (l *login) OnLostPassword(ctx app.Context, e app.Event) {
	app.Log("Lost Password Pressed")
	l.mode = "lostpassword"
	l.Update()
}

func (l *login) OnLoginButtonPress(ctx app.Context, e app.Event) {
	app.Log("Button pressed")
	go func() {
		type loginStruct struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		var login loginStruct
		login.Username = l.email
		login.Password = l.password
		data, err := json.Marshal(&login)
		if err != nil {
			app.Log(err.Error())
		}
		req, _ := http.NewRequest("POST", auth+"login", bytes.NewBuffer(data))
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			app.Log("response error")
			app.Log(err.Error())
			app.Dispatch(func() {
				loggedIn.UnSet()
				app.Navigate("/")
				l.Update()
			})
			return
		}
		app.Log("RESPONSE CODE: ", resp.Status)
		if resp.StatusCode == http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				app.Log("Problem reading response")
				app.Log(err.Error())
				app.Dispatch(func() {
					loggedIn.UnSet()
					app.Navigate("/")
					l.Update()
				})
				return
			}
			//fmt.Print("BODY:")
			//app.Log(string(body))
			var ident AuthManager
			err = json.Unmarshal(body, &ident)
			if err != nil {
				app.Log("Problem Unmarshalling response")
				app.Log(err.Error())
				app.Dispatch(func() {
					loggedIn.UnSet()
					l.loginValidating.UnSet()
					//app.Navigate("/")
					l.Update()
				})
				return
			}
			app.Log("Expires: ", ident.AuthenticationResult.ExpiresIn)
			setLocal(aToken, ident.AuthenticationResult.AccessToken)
			setLocal(rToken, ident.AuthenticationResult.RefreshToken)
			setLocal(uname, l.email)
			ident.SetExpire(ident.AuthenticationResult.ExpiresIn)
			app.Log(getLocalString(aToken))
			ident.Start()

			app.Dispatch(func() {
				app.Log("Login Success")
				loggedIn.SetTo(true)
				app.Navigate("/")
				l.Update()
			})
		} else if resp.StatusCode == http.StatusUnauthorized {
			loggedIn.UnSet()
			app.Dispatch(func() {
				l.loginValidating.UnSet()
				app.Log("Unauthorized")
				l.Update()
			})

		} else {
			loggedIn.UnSet()
			app.Dispatch(func() {
				l.loginValidating.UnSet()
				app.Log("Some Other response")
				l.Update()
			})
		}

	}()
	l.loginValidating.SetTo(true)
	l.Update()
}

func (l *login) OnBackPress(ctx app.Context, e app.Event) {
	l.mode = ""
	l.Update()
}
