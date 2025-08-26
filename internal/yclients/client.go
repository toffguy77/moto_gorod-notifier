package yclients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/thatguy/moto_gorod-notifier/internal/logger"
)

// Client is a client for interacting with YCLIENTS API.
type Client struct {
	login        string
	password     string
	partnerToken string
	userToken    string
	tokenExp     time.Time
	companyID    string
	formID       string

	http    *http.Client
	baseURL *url.URL
	log     *logger.Logger
	mu      sync.RWMutex
}

// --- Typed response models and helpers (based on provided samples) ---

type apiObject[T any] struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Attributes T      `json:"attributes"`
}

type apiResponse[T any] struct {
	Data []apiObject[T] `json:"data"`
}

type StaffAttributes struct {
	IsBookable bool    `json:"is_bookable"`
	PriceMin   float64 `json:"price_min"`
	PriceMax   float64 `json:"price_max"`
}

type DateAttributes struct {
	Date       string `json:"date"`
	IsBookable bool   `json:"is_bookable"`
}

type TimeslotAttributes struct {
	Datetime   string `json:"datetime"`
	Time       string `json:"time"`
	IsBookable bool   `json:"is_bookable"`
}

func parseStaffIDs(data []byte) ([]int, error) {
	var resp apiResponse[StaffAttributes]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse staff: %w", err)
	}
	ids := make([]int, 0, len(resp.Data))
	for _, it := range resp.Data {
		if !it.Attributes.IsBookable {
			continue
		}
		// id comes as string in response
		var sid int
		if _, err := fmt.Sscanf(it.ID, "%d", &sid); err != nil {
			continue
		}
		ids = append(ids, sid)
	}
	return ids, nil
}

func parseDates(data []byte) ([]string, error) {
	var resp apiResponse[DateAttributes]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse dates: %w", err)
	}
	out := make([]string, 0, len(resp.Data))
	for _, it := range resp.Data {
		if it.Attributes.IsBookable && it.Attributes.Date != "" {
			out = append(out, it.Attributes.Date)
		}
	}
	return out, nil
}

func parseTimeslots(data []byte) ([]string, error) {
	var resp apiResponse[TimeslotAttributes]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse timeslots: %w", err)
	}
	out := make([]string, 0, len(resp.Data))
	for _, it := range resp.Data {
		if it.Attributes.IsBookable {
			if it.Attributes.Datetime != "" {
				out = append(out, it.Attributes.Datetime)
			} else if it.Attributes.Time != "" {
				out = append(out, it.Attributes.Time)
			}
		}
	}
	return out, nil
}

// --- Convenience methods that build payload, call, and parse ---

func (c *Client) GetBookableStaffIDs(ctx context.Context, locationID, serviceID int) ([]int, error) {
	body, err := BuildSearchStaffPayload(locationID, serviceID, nil)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.SearchStaff(ctx, body)
	if err != nil {
		return nil, err
	}
	return parseStaffIDs(raw)
}

func (c *Client) GetBookableDates(ctx context.Context, locationID, serviceID int, dateFrom, dateTo string, staffID *int) ([]string, error) {
	body, err := BuildSearchDatesPayload(locationID, serviceID, dateFrom, dateTo, staffID)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.SearchDates(ctx, body)
	if err != nil {
		return nil, err
	}
	return parseDates(raw)
}

func (c *Client) GetBookableTimeslots(ctx context.Context, locationID, serviceID int, date string, staffID int) ([]string, error) {
	body, err := BuildSearchTimeslotsPayload(locationID, serviceID, date, staffID)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.SearchTimeslots(ctx, body)
	if err != nil {
		return nil, err
	}
	return parseTimeslots(raw)
}

// --- Typed payload builders (based on provided widget payloads) ---

type payloadContext struct {
	LocationID int `json:"location_id"`
}

type attendanceServiceItem struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
}

type record struct {
	StaffID                *int                    `json:"staff_id"`
	AttendanceServiceItems []attendanceServiceItem `json:"attendance_service_items"`
}

type filterStaff struct {
	Datetime *string  `json:"datetime"`
	Records  []record `json:"records"`
}

type filterDates struct {
	DateFrom string   `json:"date_from"`
	DateTo   string   `json:"date_to"`
	Records  []record `json:"records"`
}

type filterTimeslots struct {
	Date    string   `json:"date"`
	Records []record `json:"records"`
}

type searchPayload[T any] struct {
	Context payloadContext `json:"context"`
	Filter  T              `json:"filter"`
}

// BuildSearchStaffPayload builds JSON for availability/search-staff.
func BuildSearchStaffPayload(locationID int, serviceID int, staffID *int) ([]byte, error) {
	p := searchPayload[filterStaff]{
		Context: payloadContext{LocationID: locationID},
		Filter: filterStaff{
			Datetime: nil,
			Records: []record{
				{
					StaffID: staffID,
					AttendanceServiceItems: []attendanceServiceItem{{
						Type: "service",
						ID:   serviceID,
					}},
				},
			},
		},
	}
	return json.Marshal(p)
}

// BuildSearchDatesPayload builds JSON for availability/search-dates.
func BuildSearchDatesPayload(locationID int, serviceID int, dateFrom, dateTo string, staffID *int) ([]byte, error) {
	p := searchPayload[filterDates]{
		Context: payloadContext{LocationID: locationID},
		Filter: filterDates{
			DateFrom: dateFrom,
			DateTo:   dateTo,
			Records: []record{
				{
					StaffID: staffID,
					AttendanceServiceItems: []attendanceServiceItem{{
						Type: "service",
						ID:   serviceID,
					}},
				},
			},
		},
	}
	return json.Marshal(p)
}

// BuildSearchTimeslotsPayload builds JSON for availability/search-timeslots.
func BuildSearchTimeslotsPayload(locationID int, serviceID int, date string, staffID int) ([]byte, error) {
	sid := staffID
	p := searchPayload[filterTimeslots]{
		Context: payloadContext{LocationID: locationID},
		Filter: filterTimeslots{
			Date: date,
			Records: []record{
				{
					StaffID: &sid,
					AttendanceServiceItems: []attendanceServiceItem{{
						Type: "service",
						ID:   serviceID,
					}},
				},
			},
		},
	}
	return json.Marshal(p)
}

// makeRequest is a common method for making HTTP requests to YCLIENTS API
func (c *Client) makeRequest(ctx context.Context, endpoint string, body []byte) ([]byte, *http.Response, error) {
	if c.http == nil || c.baseURL == nil {
		return nil, nil, fmt.Errorf("yclients: http client not initialized")
	}

	rel, _ := url.Parse(endpoint)
	fullURL := c.baseURL.ResolveReference(rel).String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("yclients: build request: %w", err)
	}

	token, err := c.getToken(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get auth token: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+c.partnerToken+", User "+token)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.partnerToken)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("X-YCLIENTS-Application-Name", "client.booking")
	req.Header.Set("X-YCLIENTS-Application-Action", "company")
	req.Header.Set("X-YCLIENTS-Application-Platform", "go-client")

	c.log.DebugWithFields("Sending request to YCLIENTS API", logger.Fields{
		"endpoint":  fullURL,
		"body_size": len(body),
	})

	start := time.Now()
	resp, err := c.http.Do(req)
	dur := time.Since(start).Truncate(time.Millisecond)

	if err != nil {
		c.log.ErrorWithFields("YCLIENTS request failed", logger.Fields{
			"endpoint": fullURL,
			"duration": dur.String(),
			"error":    err.Error(),
		})
		return nil, resp, fmt.Errorf("yclients: request failed after %s: %w", dur, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		c.log.ErrorWithFields("Failed to read response body", logger.Fields{
			"endpoint": fullURL,
			"error":    readErr.Error(),
		})
		return nil, resp, fmt.Errorf("yclients: read body: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.log.WarnWithFields("YCLIENTS API returned non-2xx status", logger.Fields{
			"endpoint":  fullURL,
			"status":    resp.StatusCode,
			"duration":  dur.String(),
			"body":      truncateForLog(data, 600),
			"body_size": len(data),
		})
		return data, resp, fmt.Errorf("yclients: non-2xx status %d", resp.StatusCode)
	}

	c.log.DebugWithFields("YCLIENTS API request successful", logger.Fields{
		"endpoint":  fullURL,
		"status":    resp.StatusCode,
		"duration":  dur.String(),
		"body_size": len(data),
	})
	return data, resp, nil
}

// SearchStaff posts to /api/v1/b2c/booking/availability/search-staff.
func (c *Client) SearchStaff(ctx context.Context, body []byte) ([]byte, *http.Response, error) {
	return c.makeRequest(ctx, "/api/v1/b2c/booking/availability/search-staff", body)
}

// SearchDates posts to /api/v1/b2c/booking/availability/search-dates.
func (c *Client) SearchDates(ctx context.Context, body []byte) ([]byte, *http.Response, error) {
	return c.makeRequest(ctx, "/api/v1/b2c/booking/availability/search-dates", body)
}

// SearchTimeslots posts to /api/v1/b2c/booking/availability/search-timeslots.
func (c *Client) SearchTimeslots(ctx context.Context, body []byte) ([]byte, *http.Response, error) {
	return c.makeRequest(ctx, "/api/v1/b2c/booking/availability/search-timeslots", body)
}

// SearchTimes posts to /api/v1/b2c/booking/availability/search-times.
func (c *Client) SearchTimes(ctx context.Context, body []byte) ([]byte, *http.Response, error) {
	return c.makeRequest(ctx, "/api/v1/b2c/booking/availability/search-times", body)
}

func New(login, password, partnerToken, companyID, formID string) *Client {
	u, _ := url.Parse("https://platform.yclients.com")
	log := logger.New().WithField("component", "yclients_client")
	
	return &Client{
		login:        login,
		password:     password,
		partnerToken: partnerToken,
		companyID:    companyID,
		formID:       formID,
		http:         &http.Client{Timeout: 10 * time.Second},
		baseURL:      u,
		log:          log,
	}
}

// Status describes current client configuration for debugging purposes.
type Status struct {
	AuthConfigured bool
	CompanyID      string
	FormID         string
	Notes          string
}

type AuthResponse struct {
	Data struct {
		ID                int    `json:"id"`
		UserToken         string `json:"user_token"`
		Name              string `json:"name"`
		Phone             string `json:"phone"`
		Login             string `json:"login"`
		Email             string `json:"email"`
		Avatar            string `json:"avatar"`
		IsApproved        bool   `json:"is_approved"`
		IsEmailConfirmed  bool   `json:"is_email_confirmed"`
	} `json:"data"`
	Success bool `json:"success"`
}

func (c *Client) authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if time.Now().Before(c.tokenExp) {
		return nil
	}
	
	c.log.Debug("Authenticating with YCLIENTS API")
	
	endpoint := "https://api.yclients.com/api/v1/auth"
	
	payload := map[string]string{
		"login":    c.login,
		"password": c.password,
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal auth payload: %w", err)
	}
	
	c.log.DebugWithFields("Sending auth request", logger.Fields{"endpoint": endpoint})
	
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create auth request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.api.v2+json")
	req.Header.Set("Authorization", "Bearer "+c.partnerToken)
	
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}
	
	if resp.StatusCode != 201 {
		c.log.WarnWithFields("Auth request failed", logger.Fields{
			"status": resp.StatusCode,
			"body":   truncateForLog(respBody, 300),
		})
		
		// Try to parse error response for more details
		var errorResp map[string]interface{}
		if json.Unmarshal(respBody, &errorResp) == nil {
			if meta, ok := errorResp["meta"].(map[string]interface{}); ok {
				if msg, ok := meta["message"].(string); ok {
					return fmt.Errorf("auth failed: %s", msg)
				}
			}
		}
		return fmt.Errorf("auth failed with status %d", resp.StatusCode)
	}
	
	var authResp AuthResponse
	if err := json.Unmarshal(respBody, &authResp); err != nil {
		return fmt.Errorf("parse auth response: %w", err)
	}
	
	if !authResp.Success || authResp.Data.UserToken == "" {
		return fmt.Errorf("auth unsuccessful: no user token")
	}
	
	c.userToken = authResp.Data.UserToken
	c.tokenExp = time.Now().Add(4*time.Minute + 30*time.Second) // 4.5 min to refresh before expiry
	
	c.log.InfoWithFields("Successfully authenticated", logger.Fields{
		"user_id":          authResp.Data.ID,
		"user_name":        authResp.Data.Name,
		"token_expires_in": "5m",
	})
	
	return nil
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	if err := c.authenticate(ctx); err != nil {
		return "", err
	}
	
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.userToken, nil
}

// GetStatus returns a summary of current configuration, useful for logs.
func (c *Client) GetStatus(ctx context.Context) Status {
	_ = ctx
	s := Status{
		AuthConfigured: c.login != "" && c.password != "" && c.partnerToken != "",
		CompanyID:      c.companyID,
		FormID:         c.formID,
		Notes:          "full client with login/password auth",
	}
	return s
}

// HasNewSlots simulates checking for new available time slots.
func (c *Client) HasNewSlots(ctx context.Context) (bool, string, error) {
	start := time.Now()
	c.log.InfoWithFields("Starting slot availability check", logger.Fields{
		"company_id":    c.companyID,
		"form_id":       c.formID,
		"auth_set":      c.userToken != "",
	})
	
	defer func() {
		c.log.InfoWithFields("Slot availability check completed", logger.Fields{
			"duration": time.Since(start).Truncate(time.Millisecond).String(),
		})
	}()

	_ = ctx
	// Placeholder behavior: no new slots
	desc := fmt.Sprintf("stub @ %s", time.Now().Format("15:04:05"))
	return false, desc, nil
}

// truncateForLog returns a compact preview for logging error responses.
func truncateForLog(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	s := string(b)
	// Sanitize for log injection prevention
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}