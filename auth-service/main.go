package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/dgrijalva/jwt-go"
	jsoniter "github.com/json-iterator/go"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
)

const (
	flowUsernamePassword = "USER_PASSWORD_AUTH"
	flowRefreshToken     = "REFRESH_TOKEN_AUTH"
)

var (
	region   string
	log      = logrus.New()
	json     = jsoniter.ConfigCompatibleWithStandardLibrary
	poolId   string
	clientId string
	poolKey  string
	keySet   *jwk.Set
	debug    bool
)

// CognitoFlow holds internals for auth flow.s
type CognitoFlow struct {
	CognitoClient *cognitoidentityprovider.CognitoIdentityProvider
	RegFlow       *regFlow
	UserPoolID    string
	AppClientID   string
}

type regFlow struct {
	Username string
}

func init() {
	log.SetFormatter(&logrus.JSONFormatter{})
	flag.StringVar(&region, "region", "us-east-1", "Set cognito IDP Region")
	flag.Parse()
	if os.Getenv("COGNITO_USER_POOL_ID") == "" || os.Getenv("COGNITO_APP_CLIENT_ID") == "" {
		log.Fatal("Cognito Pool Information not set, make sure you set both COGNITO_USER_POOL_ID and COGNITO_APP_CLIENT_ID")
	}
	poolId = os.Getenv("COGNITO_USER_POOL_ID")
	clientId = os.Getenv("COGNITO_APP_CLIENT_ID")
	if os.Getenv("REGION") != "" {
		region = os.Getenv("REGION")
	}
	if os.Getenv("SERVER_MODE") == "DEBUG" {
		log.Info("Turning on DEBUG Mode")
		debug = true
	}
	poolKey = "https://cognito-idp." + region + ".amazonaws.com/" + poolId + "/.well-known/jwks.json"
}

func main() {
	log.Info("Starting Up")
	log.Info(poolId)
	log.Info(poolKey)
	conf := &aws.Config{Region: &region}
	sess, err := session.NewSession(conf)
	if err != nil {
		log.Fatal(err)
	}

	keySet, err = jwk.Fetch(poolKey)
	if err != nil {
		log.Fatal(err)
	}

	c := CognitoFlow{
		CognitoClient: cognitoidentityprovider.New(sess),
		RegFlow:       &regFlow{},
		UserPoolID:    poolId,
		AppClientID:   clientId,
	}

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		c.Register(w, r)
	})

	http.HandleFunc("/otp", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		c.OTP(w, r)
	})

	http.HandleFunc("/username", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		c.Username(w, r)
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		c.Login(w, r)
	})

	http.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		ValidateAccessToken(w, r)
	})

	http.HandleFunc("/signout", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		c.Signout(w, r)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		ValidateAccessToken(w, r)
	})

	http.HandleFunc("/healthz", HealthCheck)

	http.ListenAndServe(":7777", nil)
}

//HealthCheck K8S healthcheck endpoint
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func checkMethod(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}
	return true
}

// OTP handles code verification step.
func (c *CognitoFlow) OTP(w http.ResponseWriter, r *http.Request) {
	type otpdata struct {
		OTP      string `json:"otp"`
		Username string `json:"username"`
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var otp otpdata
	err = json.Unmarshal(body, &otp)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	user := &cognitoidentityprovider.ConfirmSignUpInput{
		ConfirmationCode: &otp.OTP,
		Username:         &otp.Username,
		ClientId:         aws.String(c.AppClientID),
	}

	_, err = c.CognitoClient.ConfirmSignUp(user)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusNotAcceptable)
		return
	}
}

// Register handles sign up scenario.
func (c *CognitoFlow) Register(w http.ResponseWriter, r *http.Request) {
	type userdata struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var userd userdata
	err = json.Unmarshal(body, &userd)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	username := userd.Username
	log.Infof("username: %v", username)
	password := userd.Password
	if len(password) > 0 {
		log.Infof("password is %d characters long", len(password))
	}
	email := userd.Email
	log.Infof("email: %v", email)

	user := &cognitoidentityprovider.SignUpInput{
		Username: &username,
		Password: &password,
		ClientId: aws.String(c.AppClientID),
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("email"),
				Value: &email,
			},
		},
	}

	output, err := c.CognitoClient.SignUp(user)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	/*if !*output.UserConfirmed {
		log.Error("user registration failed")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}*/
	data, err := json.Marshal(output)
	if err != nil {
		log.Error("user registration failed")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// Username handles username scenario.
func (c *CognitoFlow) Username(w http.ResponseWriter, r *http.Request) {
	type userdata struct {
		Username string `json:"username"`
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var userd userdata
	err = json.Unmarshal(body, &userd)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	_, err = c.CognitoClient.AdminGetUser(&cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(c.UserPoolID),
		Username:   &userd.Username,
	})

	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if ok {
			if awsErr.Code() == cognitoidentityprovider.ErrCodeUserNotFoundException {
				log.Info("Username %s is free", &userd.Username)
				return
			}
		} else {
			log.Error(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	log.Info("Username %s is taken.", &userd.Username)
	http.Error(w, "taken", http.StatusConflict)
}

// Login handles login scenario.
func (c *CognitoFlow) Login(w http.ResponseWriter, r *http.Request) {
	if debug {
		log.Info("Starting Login Flow")
	}
	type userdata struct {
		Username     string `json:"username"`
		Password     string `json:"password,omitempty"`
		RefreshToken string `json:"refresh_token,omitempty"`
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		if debug {
			fmt.Println(string(body))
		}
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if debug {
		fmt.Println(string(body))
	}
	var user userdata
	err = json.Unmarshal(body, &user)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Info(user.Username)

	flow := aws.String(flowUsernamePassword)
	params := map[string]*string{
		"USERNAME": aws.String(user.Username),
		"PASSWORD": aws.String(user.Password),
	}

	if user.RefreshToken != "" {
		flow = aws.String(flowRefreshToken)
		params = map[string]*string{
			"REFRESH_TOKEN": &user.RefreshToken,
		}
	}

	authTry := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow:       flow,
		AuthParameters: params,
		ClientId:       aws.String(c.AppClientID),
	}

	res, err := c.CognitoClient.InitiateAuth(authTry)
	if err != nil {
		log.Error(err)
		awsErr, ok := err.(awserr.Error)
		if ok {
			if awsErr.Code() == cognitoidentityprovider.ErrCodeResourceNotFoundException {
				http.Error(w, "", http.StatusNotFound)
			}
			if awsErr.Code() == cognitoidentityprovider.ErrCodeInvalidParameterException {
				http.Error(w, "", http.StatusInternalServerError)
			}
		}
		return
	}
	data, err := json.Marshal(&res)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	fmt.Println(string(data))
	//w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		log.Error(err)
	}
}

//ValidateAccessToken verifies that the access token used is valid
func ValidateAccessToken(w http.ResponseWriter, r *http.Request) {
	type tokendata struct {
		Accestoken string `json:"access_token"`
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var token tokendata
	err = json.Unmarshal(body, &token)
	if err != nil {
		log.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	_, err = jwt.Parse(token.Accestoken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found")
		}
		keys := keySet.LookupKeyID(kid)
		if len(keys) == 0 {
			return nil, fmt.Errorf("key %v not found", kid)
		}
		// keys[0].Materialize() doesn't exist anymore
		var raw interface{}
		return raw, keys[0].Raw(&raw)
	})
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusForbidden)
		return
	}
}

//Signout  sign out of the platform
func (c *CognitoFlow) Signout(w http.ResponseWriter, r *http.Request) {
	type tokendata struct {
		Accesstoken string `json:"access_token"`
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	var token tokendata
	err = json.Unmarshal(body, &token)
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	_, err = c.CognitoClient.GlobalSignOut(&cognitoidentityprovider.GlobalSignOutInput{
		AccessToken: &token.Accesstoken,
	})

	if err != nil {
		log.Error(err)
		awsErr, ok := err.(awserr.Error)
		if ok {
			switch awsErr.Code() {
			case cognitoidentityprovider.ErrCodeResourceNotFoundException:
				http.Error(w, "", http.StatusNotFound)
				return
			case cognitoidentityprovider.ErrCodeNotAuthorizedException:
				http.Error(w, "", http.StatusUnauthorized)
				return
			case cognitoidentityprovider.ErrCodeTooManyRequestsException:
				http.Error(w, "", http.StatusTooManyRequests)
				return
			default:
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
		}
	}
	return
}
