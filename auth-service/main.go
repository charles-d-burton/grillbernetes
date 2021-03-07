package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis"
	jsoniter "github.com/json-iterator/go"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
)

const (
	flowUsernamePassword = "USER_PASSWORD_AUTH"
	flowRefreshToken     = "REFRESH_TOKEN_AUTH"
)

var (
	region           string
	log              = logrus.New()
	rc               *redis.Client
	json             = jsoniter.ConfigCompatibleWithStandardLibrary
	poolId           string
	clientId         string
	poolKey          string
	keySet           *jwk.Set
	debug            bool
	userCache        = make(chan *cognitoidentityprovider.AuthenticationResultType, 5000)
	userInvalidation = make(chan string, 5000)
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
	var redisHost string
	log.SetFormatter(&logrus.JSONFormatter{})
	flag.StringVar(&region, "region", "us-east-1", "Set cognito IDP Region")
	flag.StringVar(&redisHost, "rh", "", "-rh Redis Hostname")
	flag.StringVar(&redisHost, "redis-host", "", "--redis-host Redis Hostname")
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
	if redisHost == "" {
		redisHost = os.Getenv("REDIS_HOST")
		if redisHost == "" {
			log.Fatal("Redis Host Undefined, exiting...")
		}

	}

	rc = redis.NewClient(&redis.Options{
		Addr:         redisHost,
		Password:     "",
		DB:           2,
		MinIdleConns: 1,
		MaxRetries:   5,
	})
	rc.Ping()
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
	res, err := rc.Ping().Result()
	if err != nil || res != "PONG" {
		log.Error("redis connection failed")
		var failure = map[string]string{"redis": "connection failed"}
		data, _ := json.Marshal(&failure)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(data)
		return
	}
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

type userdata struct {
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Login handles login scenario.
func (c *CognitoFlow) Login(w http.ResponseWriter, r *http.Request) {
	if debug {
		log.Info("Starting Login Flow")
	}
	var user userdata
	username := r.Header.Get("username")
	password := r.Header.Get("password")
	refreshToken := r.Header.Get("Authorization")
	if (username != "" && password != "") || refreshToken != "" {
		if username != "" && password != "" {
			user.Username = username
			user.Password = password
		}
		if refreshToken != "" {
			user.RefreshToken = refreshToken
		}
	} else {
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

		err = json.Unmarshal(body, &user)
		if err != nil {
			log.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	log.Info(user.Username)
	data, status, err := user.Auth(c)
	if err != nil {
		log.Error(err)
		http.Error(w, "", status)
		return
	}
	if status != http.StatusOK {
		log.Error(fmt.Errorf("Service Error Status Code: %d", status))
		http.Error(w, "", status)
		return
	}
	w.Write(data)
}

//Auth perform the user auth returning the auth response, http code, and/or an error
func (user *userdata) Auth(c *CognitoFlow) ([]byte, int, error) {
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
				return nil, http.StatusNotFound, err
			}
			if awsErr.Code() == cognitoidentityprovider.ErrCodeInvalidParameterException {
				return nil, http.StatusInternalServerError, err
			}
		}
		return nil, http.StatusUnauthorized, nil
	}
	data, err := json.Marshal(&res)
	if err != nil {
		log.Error(err)
		return nil, http.StatusInternalServerError, err
	}
	fmt.Println(string(data))
	//w.Header().Set("Content-Type", "application/json")
	if err != nil {
		log.Error(err)
	}
	//TODO: Might not be the best idea
	go func() {
		userCache <- res.AuthenticationResult
	}()
	return data, http.StatusOK, nil
}

type tokendata struct {
	Accestoken string `json:"access_token"`
}

//ValidateAccessToken verifies that the access token used is valid
func ValidateAccessToken(w http.ResponseWriter, r *http.Request) {
	var token tokendata
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Error(err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		err = json.Unmarshal(body, &token)
		if err != nil {
			log.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	token.Accestoken = authHeader
	_, err := token.Validate()
	if err != nil {
		log.Error(err)
		http.Error(w, "", http.StatusForbidden)
		return
	}
}

func (token *tokendata) Validate() (string, error) {
	tok, err := jwt.Parse(token.Accestoken, func(token *jwt.Token) (interface{}, error) {
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
	if err != nil || !tok.Valid {
		log.Error(err)
		if err == nil {
			return "", fmt.Errorf("Token validity: %t", tok.Valid)
		}
		return "", err
	}
	if claims, ok := tok.Claims.(jwt.MapClaims); ok {
		exp, ok := claims["exp"].(int64)
		if !ok {
			return "", fmt.Errorf("Token validity: %t", ok)
		}

		if exp > time.Now().Unix() {
			log.Info("Token epxired")
			return "", fmt.Errorf("Token expired %d", exp)
		}

		sub, ok := claims["sub"].(string)
		if !ok {
			return "", fmt.Errorf("Token missing sub")
		}
		return sub, nil
	}
	return "", fmt.Errorf("Something went wrong parsing token")
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

func setAuthResultToRedis() {
	for {
		select {
		case res := <-userCache:

			exp := time.Duration(*res.ExpiresIn)
			var td tokendata
			td.Accestoken = *res.AccessToken
			sub, err := td.Validate()
			if err != nil {
				log.Error(err)
				continue
			}
			rc.Set(sub, *res.RefreshToken, exp)
			//TODO: Get the sub and set the refresh token
			//rc.Set(*res.AccessToken, res, exp)
		}
	}
}

//TODO: Make the timer configurable
func validateCache() {
	ticker := time.NewTicker(5 * time.Minute)
	for {
		select {
		case <-ticker.C:

		}
	}
}
