/*
 *  Copyright (c) 2019, WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
 */

package broker

import (
	"bytes"
	"code.cloudfoundry.org/lager"
	"fmt"
	"github.com/pkg/errors"
	"github.com/wso2/service-broker-apim/pkg/client"
	"github.com/wso2/service-broker-apim/pkg/constants"
	"github.com/wso2/service-broker-apim/pkg/utils"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// BasicCredentials represents the username and Password
type BasicCredentials struct {
	Username string
	Password string
}

// DynamicClientRegReqBody represents the message body for Dynamic client request body
type DynamicClientRegReqBody struct {
	CallbackUrl string `json:"callbackUrl"`
	ClientName  string `json:"clientName"`
	Owner       string `json:"owner"`
	GrantType   string `json:"grantType"`
	SaasApp     bool   `json:"saasApp"`
}

// DynamicClientRegResBody represents the message body for Dynamic client response body
type DynamicClientRegResBody struct {
	CallbackUrl       string `json:"callBackURL"`
	JsonString        string `json:"jsonString"`
	ClientName        string `json:"clientName"`
	ClientId          string `json:"clientId"`
	ClientSecret      string `json:"clientSecret"`
	IsSaasApplication bool   `json:"isSaasApplication"`
}

// TokenResp represents the message body of the token api response
type TokenResp struct {
	Scope        string `json:"scope"`
	TokenTypes   string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

// tokens represent the Access token & Refresh token for a particular scope
type tokens struct {
	lock      sync.RWMutex //ensures atomic writes to following fields
	aT        string
	rT        string
	expiresIn time.Time
}

// TokenManager is used to manage Access token & Refresh token
type TokenManager struct {
	once                  sync.Once
	holder                map[string]*tokens
	clientID              string
	clientSec             string
	TokenEndpoint         string
	DynamicClientEndpoint string
	UserName              string
	Password              string
}

const (
	SecondSuffix                  = "s"
	ErrMSGNotEnoughArgs           = "At least one scope should be present"
	ErrMSGUnableToGetClientCreds  = "Unable to get Client credentials"
	ErrMSGUnableToGetAccessToken  = "Unable to get access token for scope: %s"
	ErrMSGUnableToParseExpireTime = "Unable parse expiresIn time"
	GenerateAccessToken           = "Generating access Token"
	DynamicClientRegMSG           = "Dynamic Client Reg"
	RefreshToken                  = "Refresh token"
	DynamicClientContext          = "/client-registration/v0.14/register"
	TokenContext                  = "/token"
)

// InitTokenManager initialize the Token Manager. This method runs only once.
// Must run before using the TokenManager
func (tm *TokenManager) InitTokenManager(scopes ...string) {
	tm.once.Do(func() {
		if len(scopes) == 0 {
			utils.HandleErrorWithLoggerAndExit(ErrMSGNotEnoughArgs, nil)
		}

		var errDynamic error
		tm.clientID, tm.clientSec, errDynamic = tm.DynamicClientReg(DefaultClientRegBody())
		if errDynamic != nil {
			utils.HandleErrorWithLoggerAndExit(ErrMSGUnableToGetClientCreds, errDynamic)
		}

		tm.holder = make(map[string]*tokens)
		for _, scope := range scopes {
			data := tm.accessTokenReqBody(scope)
			aT, rT, expiresIn, err := tm.genToken(data, GenerateAccessToken)
			if err != nil {
				utils.HandleErrorWithLoggerAndExit(fmt.Sprintf(ErrMSGUnableToGetAccessToken, scope), err)
			}
			// Handling the expire time of the access token
			duration, err := time.ParseDuration(strconv.Itoa(expiresIn) + "s")
			if err != nil {
				utils.HandleErrorWithLoggerAndExit(ErrMSGUnableToParseExpireTime, err)
			}
			tm.holder[scope] = &tokens{
				aT:        aT,
				rT:        rT,
				expiresIn: time.Now().Add(duration),
			}
		}
	})
}

// accessTokenReqBody functions returns access token request body
func (tm *TokenManager) accessTokenReqBody(scope string) url.Values {
	data := url.Values{}
	data.Set(constants.UserName, tm.UserName)
	data.Add(constants.Password, tm.Password)
	data.Add(constants.GrantType, constants.GrantPassword)
	data.Add(constants.Scope, scope)
	return data
}

// refreshTokenReqBody functions returns refresh token request body
func refreshTokenReqBody(rT string) url.Values {
	data := url.Values{}
	data.Add(constants.RefreshToken, rT)
	data.Add(constants.GrantType, constants.GrantRefreshToken)
	return data
}

// isExpired function returns true if the difference between the current time and given time is 10s
func isExpired(expiresIn time.Time) bool {
	if time.Now().Sub(expiresIn) > (10 * time.Second) {
		return true
	}
	return false
}

// Token method returns a valid Access token. If the Access token is invalid then it will regenerate a Access token
// with the Refresh token.
func (tm *TokenManager) Token(scope string) (string, error) {
	t := tm.holder[scope]
	t.lock.RLock()
	if !isExpired(t.expiresIn) {
		aT := t.aT
		t.lock.RUnlock()
		return aT, nil
	}
	t.lock.RUnlock()
	t.lock.Lock()
	if !isExpired(t.expiresIn) {
		t.lock.Unlock()
		return t.aT, nil
	}
	aT, rT, expiresIn, err := tm.refreshToken(t.rT)
	if err != nil {
		t.lock.Unlock()
		return "", err
	}
	//Parse time to type time.Duration
	duration, err := time.ParseDuration(strconv.Itoa(expiresIn) + SecondSuffix)
	utils.LogDebug("token details", &utils.LogData{
		Data: lager.Data{
			"access token":  aT,
			"refresh token": rT,
			"expires in":    expiresIn,
		},
	})
	tm.holder[scope] = &tokens{
		aT:        aT,
		rT:        rT,
		expiresIn: time.Now().Add(duration),
	}
	t.lock.Unlock()
	return aT, nil
}

// refreshToken function generates a new Access token and a Refresh token
func (tm *TokenManager) refreshToken(rTNow string) (aT, newRT string, expiresIn int, err error) {
	data := refreshTokenReqBody(rTNow)
	aT, rT, expiresIn, err := tm.genToken(data, RefreshToken)
	if err != nil {
		return "", "", 0, err
	}
	return aT, rT, expiresIn, nil
}

// genToken returns an Access token and a Refresh token from given params,
func (tm *TokenManager) genToken(reqBody url.Values, context string) (aT, rT string, expiresIn int, err error) {
	u, err := utils.ConstructURL(tm.TokenEndpoint, TokenContext)
	if err != nil {
		return "", "", 0, errors.Wrap(err, "cannot construct, token endpoint")
	}
	req, err := client.ToRequest(http.MethodPost, u, bytes.NewReader([]byte(reqBody.Encode())))
	if err != nil {
		return "", "", 0, errors.Wrapf(err, constants.ErrMSGUnableToCreateRequestBody,
			context)
	}
	req.R.SetBasicAuth(tm.clientID, tm.clientSec)
	req.R.Header.Add(constants.HTTPContentType, constants.ContentTypeUrlEncoded)
	var resBody TokenResp
	if err := client.Invoke(context, req, &resBody, http.StatusOK); err != nil {
		return "", "", 0, err
	}
	return resBody.AccessToken, resBody.RefreshToken, resBody.ExpiresIn, nil
}

// DynamicClientReg gets the Client ID and Client Secret
func (tm *TokenManager) DynamicClientReg(reqBody *DynamicClientRegReqBody) (clientId, clientSecret string, er error) {
	r, err := client.BodyReader(reqBody)
	if err != nil {
		return "", "", errors.Wrapf(err, constants.ErrMSGUnableToParseRequestBody, DynamicClientRegMSG)
	}
	u, err := utils.ConstructURL(tm.DynamicClientEndpoint, DynamicClientContext)
	if err != nil {
		return "", "", errors.Wrap(err, "cannot construct, Dynamic client registration endpoint")
	}
	// construct the request
	// Not using n=client.PostReq() method since here custom headers are added
	req, err := client.ToRequest(http.MethodPost, u, r)
	if err != nil {
		return "", "", errors.Wrapf(err, constants.ErrMSGUnableToCreateRequestBody, DynamicClientRegMSG)
	}
	req.R.SetBasicAuth(tm.UserName, tm.Password)
	req.R.Header.Set(constants.HTTPContentType, constants.ContentTypeApplicationJson)

	var resBody DynamicClientRegResBody
	if err := client.Invoke(DynamicClientRegMSG, req, &resBody, http.StatusOK); err != nil {
		return "", "", err
	}
	return resBody.ClientId, resBody.ClientSecret, nil
}

// DefaultClientRegBody returns a dynamic client request body with values
func DefaultClientRegBody() *DynamicClientRegReqBody {
	return &DynamicClientRegReqBody{
		CallbackUrl: constants.CallBackUrl,
		ClientName:  constants.ClientName,
		GrantType:   constants.DynamicClientRegGrantType,
		Owner:       constants.Owner,
		SaasApp:     true,
	}
}
