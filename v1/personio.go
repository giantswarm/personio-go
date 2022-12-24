package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseUrl = "https://api.personio.de/v1"

const timeOffsMaxLimit = 200

const QUERY_DATE_FORMAT = "2006-01-02"

// Error is an error with an associated status code
type Error interface {
	error
	Status() int
}

// StatusError represents an error with an associated HTTP status code
type StatusError struct {
	Err  error
	Code int
}

// Allows StatusError to satisfy the error interface
func (s StatusError) Error() string {
	return s.Err.Error()
}

// Status returns the contained HTTP status code
func (s StatusError) Status() int {
	return s.Code
}

// PersonioBool is a custom boolean that can be unmarshalled from 0/1 and false/true
type PersonioBool bool

func (bit *PersonioBool) UnmarshalJSON(data []byte) error {
	asString := string(data)
	if asString == "1" || asString == "true" {
		*bit = true
	} else if asString == "0" || asString == "false" {
		*bit = false
	} else {
		return fmt.Errorf("boolean unmarshal error: invalid input %s", asString)
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

// GetIntValue returns a pointer to the attributes value as an int64 or nil if no such value is available
func (a *Attribute) GetIntValue() *int64 {
	if a.Type == "integer" && a.Value != nil {
		switch a.Value.(type) {
		case float64:
			value := int64(a.Value.(float64))
			return &value
		}
	}
	return nil
}

// GetFloatValue returns a pointer to the attributes value as an float64 or nil if no such value is available
func (a *Attribute) GetFloatValue() *float64 {
	if (a.Type == "integer" || a.Type == "decimal") && a.Value != nil {
		switch a.Value.(type) {
		case float64:
			value := a.Value.(float64)
			return &value
		}
	}
	return nil
}

// GetStringValue returns a pointer to the attributes value as string or nil if no such value is available
func (a *Attribute) GetStringValue() *string {
	if (a.Type == "standard" || a.Type == "multiline") && a.Value != nil {
		switch a.Value.(type) {
		case string:
			value := a.Value.(string)
			return &value
		}
	}
	return nil
}

// GetListValue returns a pointer to the attributes value as string slice or nil if no such value is available
func (a *Attribute) GetListValue() *[]string {
	if a.Type == "list" && a.Value != nil {
		switch a.Value.(type) {
		case string:
			value := strings.FieldsFunc(a.Value.(string), func(char rune) bool { return char == ',' })
			return &value
		}
	}
	return nil
}

// GetTimeValue returns a pointer to the attributes value as time.Time or nil if no such value is available
func (a *Attribute) GetTimeValue() *time.Time {
	if a.Type == "date" && a.Value != nil {
		switch a.Value.(type) {
		case string:
			value, err := time.Parse(time.RFC3339, a.Value.(string))
			if err == nil {
				return &value
			}
		case time.Time:
			value := a.Value.(time.Time)
			return &value
		}
	}
	return nil
}

// GetMapValue returns a pointer to the embedded objects attributes as map or nil if no such value is available
func (a *Attribute) GetMapValue() map[string]interface{} {
	if a.Type == "standard" && a.Value != nil {
		nested, _ := a.Value.(map[string]interface{})
		nestedAttributes, _ := nested["attributes"].(map[string]interface{})
		return nestedAttributes
	}
	return map[string]interface{}{}
}

// AttributeContainer is something that has object attributes of the elaborate and/or dynamic kind
type AttributeContainer struct {
	Attributes map[string]Attribute `json:"attributes"`
}

// GetIntAttribute returns a pointer to the specified attributes value as int64 or nil
func (ac *AttributeContainer) GetIntAttribute(key string) *int64 {
	attr := ac.Attributes[key]
	return attr.GetIntValue()
}

// GetFloatAttribute returns a pointer to the specified attributes value as float64 or nil
func (ac *AttributeContainer) GetFloatAttribute(key string) *float64 {
	attr := ac.Attributes[key]
	return attr.GetFloatValue()
}

// GetStringAttribute returns a pointer to the specified attributes value as string or nil
func (ac *AttributeContainer) GetStringAttribute(key string) *string {
	attr := ac.Attributes[key]
	return attr.GetStringValue()
}

// GetListAttribute returns a pointer to the specified attributes value as string slice or nil
func (ac *AttributeContainer) GetListAttribute(key string) *[]string {
	attr := ac.Attributes[key]
	return attr.GetListValue()
}

// GetTimeAttribute returns a pointer to the specified attributes value as time.Time or nil
func (ac *AttributeContainer) GetTimeAttribute(key string) *time.Time {
	attr := ac.Attributes[key]
	return attr.GetTimeValue()
}

// GetMapAttribute returns a map of the nested value's attributes or an empty map
func (ac *AttributeContainer) GetMapAttribute(key string) map[string]interface{} {
	attr := ac.Attributes[key]
	return attr.GetMapValue()
}

// Employee is a single employee entry
type Employee struct {
	Type string `json:"type"`
	AttributeContainer
}

// TimeOff is a single time-off entry
type TimeOff struct {
	Id           int64        `json:"id"`
	Status       string       `json:"status"`
	StartDate    time.Time    `json:"start_date"`
	EndDate      time.Time    `json:"end_date"`
	DaysCount    float64      `json:"days_count"`
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
	Employee    Employee `json:"employee"`
	CreatedBy   string   `json:"created_by"`
	Certificate struct {
		Status string `json:"status"`
	} `json:"certificate"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EmployeeResult is the response body of /company/employee/{{id}}
type employeeResult struct {
	Data Employee `json:"data"`
}

// EmployeeResult is the response body of /company/employee/{{id}}
type employeesResult struct {
	Data []Employee `json:"data"`
}

// timeOffContainer is the typed object returned for time-offs by Personio
type timeOffContainer struct {
	Type       string  `json:"type"`
	Attributes TimeOff `json:"attributes"`
}

// timeOffsResult is the response body of /company/time-offs
type timeOffsResult struct {
	Data []timeOffContainer `json:"data"`
}

// Credentials is the secret to authenticate with the Personio API v1
type Credentials struct {
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	AccessToken  string `json:"accessToken,omitempty"`
}

// Client is a Personio API v1 instance
type Client struct {
	ctx     context.Context
	baseUrl string
	client  http.Client
	secret  Credentials
}

// NewClientWithTimeout creates a new Client instance with the specified credentials and timeout
func NewClientWithTimeout(ctx context.Context, baseUrl string, secret Credentials, timeout time.Duration) (*Client, error) {

	if baseUrl == "" {
		baseUrl = DefaultBaseUrl
	}

	return &Client{
		ctx:     ctx,
		baseUrl: baseUrl,
		client:  http.Client{Timeout: timeout},
		secret:  secret,
	}, nil
}

// NewClient creates a new Client instance with the specified Credentials
func NewClient(ctx context.Context, baseUrl string, secret Credentials) (*Client, error) {
	return NewClientWithTimeout(ctx, baseUrl, secret, time.Duration(40)*time.Second)
}

// doRequest processes the specified request, optionally handling authentication
func (personio *Client) doRequest(request *http.Request, useAuthentication bool) ([]byte, error) {

	// authenticate
	if useAuthentication && personio.secret.AccessToken == "" {
		token, err := personio.Authenticate(personio.secret.ClientId, personio.secret.ClientSecret)
		if err != nil {
			return nil, err
		}

		personio.secret.AccessToken = token
	}

	if useAuthentication && personio.secret.AccessToken != "" {
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
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(response.Body)

	if useAuthentication {
		// cycle or reset accessToken
		nextAuthorization := strings.Replace(response.Header.Get("authorization"), "Bearer ", "", 1)
		if nextAuthorization != "" {
			personio.secret.AccessToken = nextAuthorization
		}
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, StatusError{errors.New(response.Status), response.StatusCode}
	}

	var body []byte
	body, err = io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// doRequestJson processes the specified request assuming JSON data is exchanged
func (personio *Client) doRequestJson(request *http.Request, useAuthentication bool) ([]byte, error) {

	request.Header.Set("Accept", "application/json")

	body, err := personio.doRequest(request, useAuthentication)
	if err != nil {
		return nil, err
	}

	var result resultBody
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("personio returned error: code=%d, message=%s", result.Error.Code, result.Error.Message)
	}

	return body, nil
}

// Authenticate fetches a new access token for the given clientId and clientSecret
func (personio *Client) Authenticate(clientId string, clientSecret string) (string, error) {

	form := url.Values{}
	form.Add("client_id", clientId)
	form.Add("client_secret", clientSecret)

	req, err := http.NewRequest(http.MethodPost, personio.baseUrl+"/auth", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var body []byte
	body, err = personio.doRequestJson(req, false)
	if err != nil {
		return "", err
	}

	var auth Auth
	err = json.Unmarshal(body, &auth)
	if err != nil {
		return "", err
	}

	return auth.Data.Token, nil
}

// GetEmployee fetches one or multiple employees.json by optional ID
func (personio *Client) GetEmployee(id int64) (*Employee, error) {

	req, err := http.NewRequest(http.MethodGet, personio.baseUrl+fmt.Sprintf("/company/employees/%d", id), nil)
	if err != nil {
		return nil, err
	}

	body, err := personio.doRequestJson(req, true)
	if err != nil {
		return nil, err
	}

	var employeeResult employeeResult
	err = json.Unmarshal(body, &employeeResult)
	if err != nil {
		return nil, err
	}

	// unpack single Employee element
	return &employeeResult.Data, nil
}

// GetEmployees returns all employees
func (personio *Client) GetEmployees() ([]*Employee, error) {

	req, err := http.NewRequest(http.MethodGet, personio.baseUrl+"/company/employees", nil)
	if err != nil {
		return nil, err
	}

	body, err := personio.doRequestJson(req, true)
	if err != nil {
		return nil, err
	}

	var employeesResult employeesResult
	err = json.Unmarshal(body, &employeesResult)
	if err != nil {
		return nil, err
	}

	// unpack TimeOff elements
	employees := make([]*Employee, len(employeesResult.Data))
	for i := range employeesResult.Data {
		employees[i] = &employeesResult.Data[i]
	}

	return employees, nil
}

// GetTimeOffs returns the time-offs matching the specified start and end dates (inclusive, ignored if zero)
//
// Parameters offset and limit are not bound by the Personio APIs limits
func (personio *Client) GetTimeOffs(start *time.Time, end *time.Time, offset int, limit int) ([]*TimeOff, error) {

	var count = 0
	var results []timeOffsResult
	for count < limit {

		req, err := http.NewRequest(http.MethodGet, personio.baseUrl+"/company/time-offs", nil)
		if err != nil {
			return nil, err
		}

		query := req.URL.Query()
		if start != nil {
			query.Add("start_date", start.Format(QUERY_DATE_FORMAT))
		}
		if end != nil {
			query.Add("end_date", end.Format(QUERY_DATE_FORMAT))
		}

		var stepLimit = limit - count
		if stepLimit > timeOffsMaxLimit {
			stepLimit = timeOffsMaxLimit
		}
		query.Add("limit", strconv.Itoa(stepLimit))
		query.Add("offset", strconv.Itoa(offset+count))
		req.URL.RawQuery = query.Encode()

		body, err := personio.doRequestJson(req, true)
		if err != nil {
			return nil, err
		}

		var result timeOffsResult
		err = json.Unmarshal(body, &result)
		if err != nil {
			return nil, err
		}

		resultLength := len(result.Data)
		if resultLength > 0 {
			results = append(results, result)
			count += resultLength
		}

		if resultLength < stepLimit {
			break
		}
	}

	// unpack TimeOff elements
	timeOffs := make([]*TimeOff, count)
	for i := range results {
		for j := range results[i].Data {
			timeOffs[(i*timeOffsMaxLimit)+j] = &results[i].Data[j].Attributes
		}
	}

	return timeOffs, nil
}
