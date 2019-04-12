/*
 *  Copyright (c) 2019, WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
 */

package broker

import (
	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/wso2/service-broker-apim/pkg/client"
	"github.com/wso2/service-broker-apim/pkg/constants"
	"net/http"
)

const (
	CreateAPIContext = "create API"
	HeaderAuth       = "Authorization"
	HeaderBear       = "Bearer "
)

// APIMManager handles the communication with API Manager
type APIMManager struct {
	APIMEndpoint string
	InsecureCon  bool
}

// AppReq represents the application creation request body
type AppReq struct {
	ThrottlingTier string `json:"throttlingTier"`
	Description    string `json:"description"`
	Name           string `json:"name"`
	CallbackUrl    string `json:"callbackUrl"`
}

// CreateAPI function creates
func (am *APIMManager) CreateAPI(apiReqBody APIReqBody, tm *TokenManager) (string, error) {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(apiReqBody)
	req, err := http.NewRequest(http.MethodPost, am.APIMEndpoint, buf)
	aT, err := tm.Token(ScopeAPICreate)
	if err != nil {
		return "", errors.Wrap(err, CreateAPIContext)
	}
	req.Header.Add(HeaderAuth, HeaderBear+aT)
	req.Header.Set(constants.HTTPContentType, constants.ContentTypeApplicationJson)
	var resBody APIResp
	err = client.Invoke(am.InsecureCon, CreateAPIContext, req, &resBody, http.StatusCreated)
	if err != nil {
		return "", err
	}
	return resBody.Id, nil
}

