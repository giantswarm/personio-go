package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/giantswarm/microerror"
	"github.com/golang/glog"
)

const baseUrl = "https://api.personio.de/v1"

// PersonioBool is a custom boolean that can be unmarshalled from 0/1 and false/true
type PersonioBool bool

func (bit *PersonioBool) UnmarshalJSON(data []byte) error {
	asString := string(data)
	if asString == "1" || asString == "true" {
		*bit = true
	} else if asString == "0" || asString == "false" {
		*bit = false
	} else {
		return errors.New(fmt.Sprintf("Boolean unmarshal error: invalid input %s", asString))
	}
	return nil
}

// resultBody is the basic Json document returned by Personio API v1
type resultBody struct {
	Success bool `json:"success"`
	Error   struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// Auth is the response body of /auth
type Auth struct {
	Success bool `json:"success"`
	Error   struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
	Data struct {
		Token string `json:"token"`
	} `json:"data,omitempty"`
}

// Attribute is a nested Personio API v1 attribute (they have variable type and are configurable)
type Attribute struct {
	Label       string      `json:"label"`
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	UniversalId string      `json:"universal_id"`
}

// TimeOff is a single time-off entry
type TimeOff struct {
	Id           int64        `json:"id"`
	Status       string       `json:"status"`
	StartDate    time.Time    `json:"start_date"`
	EndDate      time.Time    `json:"end_date"`
	DaysCount    int          `json:"days_count"`
	HalfDayStart PersonioBool `json:"half_day_start"`
	HalfDayEnd   PersonioBool `json:"half_day_end"`
	TimeOffType  struct {
		Type       string `json:"type"`
		Attributes struct {
			Id       int64  `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
		} `json:"attributes"`
	} `json:"time_off_type"`
	Employee struct {
		Type       string `json:"type"`
		Attributes map[string]Attribute
	} `json:"employee"`
	CreatedBy   string `json:"created_by"`
	Certificate struct {
		Status string `json:"status"`
	} `json:"certificate"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// timeOffResult is the response body of /company/time-offs
type timeOffResult struct {
	Success bool `json:"success"`
	Error   struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error,omitempty"`
	Data []struct {
		Type       string  `json:"type"`
		Attributes TimeOff `json:"attributes"`
	} `json:"data,omitempty"`
}

// Credentials is the secret to authenticate with the Personio API v1
type Credentials struct {
	ClientId     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	AccessToken  string `json:"accessToken,omitempty"`
}

// Client is a Personio API v1 instance
type Client struct {
	ctx    context.Context
	client http.Client
	secret Credentials
}

// NewClient creates a new Client instance with the specified Credentials
func NewClient(ctx context.Context, secret Credentials) (*Client, error) {
	return &Client{
		ctx:    ctx,
		client: http.Client{Timeout: time.Duration(10) * time.Second},
		secret: secret,
	}, nil
}

// doRequest processes the specified request, optionally handling authentication
func (personio *Client) doRequest(request *http.Request, useAuthorization bool) ([]byte, error) {

	// authorize
	if useAuthorization && personio.secret.AccessToken == "" {
		auth, err := personio.Authorize(personio.secret.ClientId, personio.secret.ClientSecret)
		if err != nil {
			return nil, microerror.Mask(err)
		}

		personio.secret.AccessToken = auth.Data.Token
	}

	if useAuthorization && personio.secret.AccessToken != "" {
		(*request).Header.Set("Authorization", "Bearer "+personio.secret.AccessToken)
		personio.secret.AccessToken = "" // token consumed
	}

	var response *http.Response
	var err error
	if personio.ctx == nil {
		response, err = personio.client.Do(request)
	} else {
		response, err = personio.client.Do(request.WithContext(personio.ctx))
		// preserve error of cancelled context
		if err != nil {
			select {
			case <-personio.ctx.Done():
				err = personio.ctx.Err()
			default:
			}
		}
	}
	if err != nil {
		return nil, microerror.Mask(err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			glog.Error("Failed to close response body: %s", err)
		}
	}(response.Body)

	if useAuthorization {
		// cycle or reset accessToken
		nextAuthorization := strings.Replace(response.Header.Get("authorization"), "Bearer ", "", 1)
		if nextAuthorization != "" {
			personio.secret.AccessToken = nextAuthorization
		}
	}

	var body []byte
	body, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return body, nil
}

// doRequestJson processes the specified request assuming JSON data is exchanged
func (personio *Client) doRequestJson(request *http.Request, useAuthorization bool) ([]byte, error) {

	request.Header.Set("Accept", "application/json")

	body, err := personio.doRequest(request, useAuthorization)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var result resultBody
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	if !result.Success {
		return nil, errors.New(fmt.Sprintf("Personio returned an error: code=%d, message=%s", result.Error.Code, result.Error.Message))
	}

	return body, nil
}

// Authorize fetches a new access token for the given clientId and clientSecret
func (personio *Client) Authorize(clientId string, clientSecret string) (*Auth, error) {

	form := url.Values{}
	form.Add("client_id", clientId)
	form.Add("client_secret", clientSecret)

	req, err := http.NewRequest(http.MethodPost, baseUrl+"/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, microerror.Mask(err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var body []byte
	body, err = personio.doRequestJson(req, false)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var auth Auth
	err = json.Unmarshal(body, &auth)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	return &auth, nil
}

// GetTimeOffs returns the time-offs for the specified start and end dates (inclusive)
func (personio *Client) GetTimeOffs(start time.Time, end time.Time) ([]*TimeOff, error) {

	req, err := http.NewRequest(http.MethodGet, baseUrl+"/company/time-offs", nil)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	query := req.URL.Query()
	query.Add("start_date", start.Format(time.RFC3339))
	query.Add("end_date", end.Format(time.RFC3339))

	body, err := personio.doRequestJson(req, true)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	var timeOffResult timeOffResult
	err = json.Unmarshal(body, &timeOffResult)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	// unpack TimeOff elements
	timeOffs := make([]*TimeOff, len(timeOffResult.Data))
	for i, entry := range timeOffResult.Data {
		timeOffs[i] = &entry.Attributes
	}

	return timeOffs, nil
}
