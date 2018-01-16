package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	eh "github.com/looplab/eventhorizon"
	domain "github.com/superchalupa/go-redfish/internal/redfishresource"
)

type LoginRequest struct {
	UserName string
	Password string
}

// This is a fairly slow implementation, but should be good enough for our
// purposes. This could be optimized to operate in about 1/5th of the time
const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz1234567890"

var moduleRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func createRandSecret(length int, characters string) []byte {
	b := make([]byte, length)
	charLen := len(characters)
	for i := range b {
		b[i] = characters[moduleRand.Intn(charLen)]
	}
	return b
}

const (
	POSTCommand = eh.CommandType("SessionService:POST")
)

// HTTP POST Command
type POST struct {
	eventBus       eh.EventBus
	commandHandler eh.CommandHandler
	ID             eh.UUID           `json:"id"`
	CmdID          eh.UUID           `json:"cmdid"`
	Headers        map[string]string `eh:"optional"`
	LR             LoginRequest
}

// Static type checking for commands to prevent runtime errors due to typos
var _ = eh.Command(&POST{})

func (c *POST) AggregateType() eh.AggregateType { return domain.AggregateType }
func (c *POST) AggregateID() eh.UUID            { return c.ID }
func (c *POST) CommandType() eh.CommandType     { return POSTCommand }
func (c *POST) SetAggID(id eh.UUID)             { c.ID = id }
func (c *POST) SetCmdID(id eh.UUID)             { c.CmdID = id }
func (c *POST) ParseHTTPRequest(r *http.Request) error {
	json.NewDecoder(r.Body).Decode(&c.LR)
	return nil
}
func (c *POST) Handle(ctx context.Context, a *domain.RedfishResourceAggregate) error {
	fmt.Printf("SPECIAL CODE TO HANDLE SESSION SERVICE POST!!\n")

	// step 1: validate username/password (PUNT FOR NOW) // TODO: implement real pam here
	if c.LR.UserName != "root" || c.LR.Password != "password" {
		return errors.New("Could not verify username/password")
	}

	// fake it for now
	privileges := []string{"read", "write"}

	// step 2: Generate new session
	sessionUUID := eh.NewUUID()
	// TODO: hardcoded /redfish/v1
	sessionURI := fmt.Sprintf("/redfish/v1/SessionService/Sessions/%s", sessionUUID)

	token := jwt.New(jwt.SigningMethodHS256)
	claims := make(jwt.MapClaims)
	//claims["exp"] = time.Now().Add(time.Hour * time.Duration(1)).Unix()
	claims["iat"] = time.Now().Unix()
	claims["iss"] = "localhost"
	claims["sub"] = c.LR.UserName
	claims["privileges"] = privileges
	claims["sessionuri"] = sessionURI
	token.Claims = claims
	secret := createRandSecret(24, characters)
	tokenString, err := token.SignedString(secret)

	retprops := map[string]interface{}{
		"@odata.type": "#Session.v1_0_0.Session",
		"@odata.id":   sessionURI,
		// TODO: hardcoded /redfish/v1
		"@odata.context": "/redfish/v1/$metadata#Session.Session",
		"Id":             fmt.Sprintf("%s", sessionUUID),
		"Name":           "User Session",
		"Description":    "User Session",
		"UserName":       c.LR.UserName,
		"x-auth-token":   tokenString, // TODO: remove this, it's only for testing
	}

	err = c.commandHandler.HandleCommand(
		ctx,
		&domain.CreateRedfishResource{
			ID:          sessionUUID,
			ResourceURI: retprops["@odata.id"].(string),
			Properties:  retprops,
			Private:     map[string]interface{}{"token_secret": secret},
		})
	if err != nil {
		return err
	}

	c.startSessionDeleteTimer(sessionUUID, sessionURI)

	c.eventBus.HandleEvent(ctx, eh.NewEvent(domain.HTTPCmdProcessed, &domain.HTTPCmdProcessedData{
		CommandID:  c.CmdID,
		Results:    retprops,
		StatusCode: 200,
		Headers:    map[string]string{"X-Auth-Token": tokenString},
	}, time.Now()))
	return nil
}

func (c *POST) startSessionDeleteTimer(sessionUUID eh.UUID, sessionURI string) {
	return
}
