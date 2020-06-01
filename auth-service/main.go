package main

import (
	"errors"
	"flag"
	"fmt"
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

// CognitoFlow holds internals for auth flow.
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if debug {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if !checkMethod(r) {
			http.Error(w, "", http.StatusBadRequest)
		}
		ValidateAccessToken(w, r)
	})

	http.ListenAndServe(":7777", nil)
}

func checkMethod(r *http.Request) bool {
	if r.Method != "POST" {
		return false
	}
	return true
}

// OTP handles code verification step.
func (c *CognitoFlow) OTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	otp := r.Form.Get("otp")

	user := &cognitoidentityprovider.ConfirmSignUpInput{
		ConfirmationCode: aws.String(otp),
		Username:         aws.String(c.RegFlow.Username),
		ClientId:         aws.String(c.AppClientID),
	}

	_, err := c.CognitoClient.ConfirmSignUp(user)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "", http.StatusNotAcceptable)
		return
	}
}

// Register handles sign up scenario.
func (c *CognitoFlow) Register(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	username := r.Form.Get("username")
	log.Infof("username: %v", username)
	password := r.Form.Get("password")
	if len(password) > 0 {
		log.Infof("password is %d characters long", len(password))
	}
	email := r.Form.Get("email")
	log.Infof("email: %v", email)

	user := &cognitoidentityprovider.SignUpInput{
		Username: aws.String(username),
		Password: aws.String(password),
		ClientId: aws.String(c.AppClientID),
		UserAttributes: []*cognitoidentityprovider.AttributeType{
			{
				Name:  aws.String("email"),
				Value: aws.String(email),
			},
		},
	}

	_, err := c.CognitoClient.SignUp(user)
	if err != nil {
		fmt.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// Username handles username scenario.
func (c *CognitoFlow) Username(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	username := r.Form.Get("username")

	_, err := c.CognitoClient.AdminGetUser(&cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(c.UserPoolID),
		Username:   aws.String(username),
	})

	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if ok {
			if awsErr.Code() == cognitoidentityprovider.ErrCodeUserNotFoundException {
				log.Info("Username %s is free", username)
				return
			}
		} else {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	log.Info("Username %s is taken.", username)
	http.Error(w, "taken", http.StatusConflict)
}

// Login handles login scenario.
func (c *CognitoFlow) Login(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	username := r.Form.Get("username")
	password := r.Form.Get("password")
	refreshToken := r.Form.Get("refresh_token")

	flow := aws.String(flowUsernamePassword)
	params := map[string]*string{
		"USERNAME": aws.String(username),
		"PASSWORD": aws.String(password),
	}

	if refreshToken != "" {
		flow = aws.String(flowRefreshToken)
		params = map[string]*string{
			"REFRESH_TOKEN": aws.String(refreshToken),
		}
	}

	authTry := &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow:       flow,
		AuthParameters: params,
		ClientId:       aws.String(c.AppClientID),
	}

	res, err := c.CognitoClient.InitiateAuth(authTry)
	if err != nil {
		fmt.Println(err)
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
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		log.Error(err)
	}
	//json.NewEncoder(w).Encode(data)
}

//ValidateAccessToken verifies that the access token used is valid
func ValidateAccessToken(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	tokenString := r.Form.Get("access_token")
	_, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
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
