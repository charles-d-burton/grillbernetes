package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/charles-d-burton/grillbernetes/gateway/graph/generated"
	"github.com/charles-d-burton/grillbernetes/gateway/graph/model"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

func (r *mutationResolver) Register(ctx context.Context, input model.NewUser) (bool, error) {
	if input.Email == "" {
		return false, errors.New("email not provided")
	}
	if input.Password == "" {
		return false, errors.New("password not provided")
	}

	url := authURL + "/username"
	values := map[string]string{
		"username": input.Email,
		"email":    input.Email,
		"password": input.Password,
	}
	data, err := json.Marshal(values)
	if err != nil {
		log.Error(err)
		return false, err
	}
	resp, err := makeReq(url, data)
	if err != nil {
		log.Error(err)
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Error(resp.StatusCode)
		return false, errors.New("recieved bad response")
	}

	url = authURL + "/register"
	resp, err = makeReq(url, data)
	if err != nil {
		log.Error(err)
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Error(resp.StatusCode)
		return false, errors.New("recieved bad response")
	}
	return true, nil
}

func (r *mutationResolver) Login(ctx context.Context, input model.Login) (*model.User, error) {
	url := authURL + "/login"
	values := map[string]string{
		"username": input.Email,
	}
	if input.Password != nil {
		values["password"] = *input.Password
	}
	if input.RefreshToken != nil {
		values["refresh_token"] = *input.RefreshToken
	}
	data, err := json.Marshal(values)
	fmt.Println(string(data))
	if err != nil {
		return nil, err
	}
	resp, err := makeReq(url, data)
	body, err := ioutil.ReadAll(resp.Body)
	log.Print("BODY:")
	log.Println(string(body))
	var ident cognitoidentityprovider.InitiateAuthOutput
	err = json.UnmarshalFromString(string(body), &ident)
	var user model.User
	user.AccessToken = ident.AuthenticationResult.AccessToken
	user.RefreshToken = ident.AuthenticationResult.RefreshToken
	user.ID = *ident.AuthenticationResult.IdToken
	return &user, nil
}

func (r *mutationResolver) SignOut(ctx context.Context, input model.Login) (bool, error) {
	if *input.AccessToken == "" {
		return false, errors.New("access token mission")
	}
	url := authURL + "/signout"
	values := map[string]string{
		"access_token": *input.AccessToken,
	}
	data, err := json.Marshal(values)
	if err != nil {
		return false, err
	}
	resp, err := makeReq(url, data)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Error(resp.StatusCode)
		return false, errors.New("recieved bad response")
	}
	return true, nil
}

func (r *mutationResolver) UserAvailable(ctx context.Context, input model.Username) (bool, error) {
	if input.Username == "" {
		return false, errors.New("username missing")
	}
	url := authURL + "/username"
	values := map[string]string{
		"username": input.Username,
	}
	data, err := json.Marshal(values)
	if err != nil {
		log.Error(err)
		return false, err
	}
	resp, err := makeReq(url, data)
	if err != nil {
		log.Error(err)
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		log.Error(resp.StatusCode)
		return false, errors.New("recieved bad response")
	}
	return true, nil
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

type mutationResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	accesstokenPlaceholder := "access-token"
	refreshtokenPlaceholder := "refresh-token"
	var user model.User
	user.ID = "charles.d.burton@gmail.com"
	user.Email = "charles.d.burton@gmail.com"
	user.AccessToken = &accesstokenPlaceholder
	user.RefreshToken = &refreshtokenPlaceholder
	return &user, nil
}
func makeReq(url string, data []byte) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data)) //This is inefficient, should change to pool of handlers with re-usable buffers
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

var (
	json    = jsoniter.ConfigCompatibleWithStandardLibrary
	log     = logrus.New()
	authURL = "https://auth.home.rsmachiner.com"
)
