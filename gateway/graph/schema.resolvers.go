package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/charles-d-burton/grillbernetes/gateway/graph/generated"
	"github.com/charles-d-burton/grillbernetes/gateway/graph/model"
	jsoniter "github.com/json-iterator/go"
)

func (r *mutationResolver) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	var user model.User
	user.ID = "charles.d.burton@gmail.com"
	user.Name = "charles.d.burton@gmail.com"
	user.AccessToken = "access-token"
	user.RefreshToken = "refresh-token"
	return &user, nil
}

func (r *mutationResolver) Login(ctx context.Context, input model.Login) (*model.User, error) {
	url := "https://auth.home.rsmachiner.com/login"
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
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	body, err := ioutil.ReadAll(resp.Body)
	log.Print("BODY:")
	log.Println(string(body))
	var ident cognitoidentityprovider.InitiateAuthOutput
	err = json.UnmarshalFromString(string(body), &ident)
	var user model.User
	user.AccessToken = *ident.AuthenticationResult.AccessToken
	user.RefreshToken = *ident.AuthenticationResult.RefreshToken
	user.ID = *ident.AuthenticationResult.IdToken
	return &user, nil
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
var json = jsoniter.ConfigCompatibleWithStandardLibrary
