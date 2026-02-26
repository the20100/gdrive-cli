package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const apiBase = "https://www.googleapis.com/drive/v3"

// RefreshFunc is called when the access token is expired.
// It returns the new access token and its Unix expiry timestamp.
type RefreshFunc func() (newToken string, expiresAt int64, err error)

// Client is an authenticated Google Drive API v3 client.
type Client struct {
	token       string
	tokenExpiry int64
	refreshFn   RefreshFunc
	httpClient  *http.Client
}

// NewClient creates an authenticated Client.
// refreshFn may be nil if no token refresh is needed (e.g. manual token set with no refresh token).
func NewClient(token string, tokenExpiry int64, refreshFn RefreshFunc) *Client {
	return &Client{
		token:       token,
		tokenExpiry: tokenExpiry,
		refreshFn:   refreshFn,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ensureToken refreshes the access token if it expires within 60 seconds.
func (c *Client) ensureToken() error {
	if c.refreshFn == nil {
		return nil
	}
	if c.tokenExpiry > 0 && time.Now().Unix() < c.tokenExpiry-60 {
		return nil
	}
	newToken, expiresAt, err := c.refreshFn()
	if err != nil {
		return fmt.Errorf("refreshing token: %w", err)
	}
	c.token = newToken
	c.tokenExpiry = expiresAt
	return nil
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if jerr := json.Unmarshal(body, &errResp); jerr == nil && errResp.Error.Message != "" {
			return nil, &DriveError{
				StatusCode: resp.StatusCode,
				Message:    fmt.Sprintf("HTTP %d: %s", errResp.Error.Code, errResp.Error.Message),
			}
		}
		return nil, &DriveError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}
	}
	return body, nil
}

// doRawRequest sends the request and returns raw bytes without setting Accept: application/json.
// Used for file downloads and exports that return binary content.
func (c *Client) doRawRequest(req *http.Request) ([]byte, http.Header, error) {
	if err := c.ensureToken(); err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, nil, &DriveError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
		}
	}
	return body, resp.Header, nil
}

func (c *Client) buildURL(path string, params url.Values) string {
	u, _ := url.Parse(apiBase + path)
	if params != nil {
		u.RawQuery = params.Encode()
	}
	return u.String()
}

func (c *Client) Get(path string, params url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.buildURL(path, params), nil)
	if err != nil {
		return nil, err
	}
	return c.doRequest(req)
}

func (c *Client) Post(path string, params url.Values, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encoding request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(http.MethodPost, c.buildURL(path, params), body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.doRequest(req)
}

func (c *Client) Patch(path string, params url.Values, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encoding request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(http.MethodPatch, c.buildURL(path, params), body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.doRequest(req)
}

func (c *Client) Delete(path string, params url.Values) error {
	req, err := http.NewRequest(http.MethodDelete, c.buildURL(path, params), nil)
	if err != nil {
		return err
	}
	_, err = c.doRequest(req)
	return err
}

// ---- File methods ----

const fileFields = "id,name,mimeType,size,createdTime,modifiedTime,parents,trashed,webViewLink,starred,shared,ownedByMe,owners"

func (c *Client) ListFiles(query string, pageToken string, pageSize int, includeTrash bool) ([]*File, string, error) {
	params := url.Values{}
	params.Set("fields", "nextPageToken,files("+fileFields+")")
	params.Set("pageSize", strconv.Itoa(pageSize))

	q := query
	if !includeTrash {
		if q != "" {
			q = "(" + q + ") and trashed=false"
		} else {
			q = "trashed=false"
		}
	}
	if q != "" {
		params.Set("q", q)
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	body, err := c.Get("/files", params)
	if err != nil {
		return nil, "", err
	}
	var resp struct {
		Files         []*File `json:"files"`
		NextPageToken string  `json:"nextPageToken"`
	}
	return resp.Files, resp.NextPageToken, json.Unmarshal(body, &resp)
}

func (c *Client) GetFile(id string) (*File, error) {
	params := url.Values{}
	params.Set("fields", fileFields)
	body, err := c.Get("/files/"+id, params)
	if err != nil {
		return nil, err
	}
	var f File
	return &f, json.Unmarshal(body, &f)
}

func (c *Client) UpdateFile(id string, payload map[string]any) (*File, error) {
	params := url.Values{}
	params.Set("fields", fileFields)
	body, err := c.Patch("/files/"+id, params, payload)
	if err != nil {
		return nil, err
	}
	var f File
	return &f, json.Unmarshal(body, &f)
}

func (c *Client) DeleteFile(id string) error {
	return c.Delete("/files/"+id, nil)
}

func (c *Client) TrashFile(id string) (*File, error) {
	params := url.Values{}
	params.Set("fields", fileFields)
	body, err := c.Patch("/files/"+id, params, map[string]bool{"trashed": true})
	if err != nil {
		return nil, err
	}
	var f File
	return &f, json.Unmarshal(body, &f)
}

func (c *Client) UntrashFile(id string) (*File, error) {
	params := url.Values{}
	params.Set("fields", fileFields)
	body, err := c.Patch("/files/"+id, params, map[string]bool{"trashed": false})
	if err != nil {
		return nil, err
	}
	var f File
	return &f, json.Unmarshal(body, &f)
}

func (c *Client) CopyFile(id string, name string) (*File, error) {
	payload := map[string]string{}
	if name != "" {
		payload["name"] = name
	}
	params := url.Values{}
	params.Set("fields", fileFields)
	body, err := c.Post("/files/"+id+"/copy", params, payload)
	if err != nil {
		return nil, err
	}
	var f File
	return &f, json.Unmarshal(body, &f)
}

// DownloadFile retrieves the raw binary content of a file.
// Returns the content bytes and the Content-Type header.
func (c *Client) DownloadFile(id string) ([]byte, string, error) {
	params := url.Values{}
	params.Set("alt", "media")
	req, err := http.NewRequest(http.MethodGet, c.buildURL("/files/"+id, params), nil)
	if err != nil {
		return nil, "", err
	}
	data, headers, err := c.doRawRequest(req)
	if err != nil {
		return nil, "", err
	}
	return data, headers.Get("Content-Type"), nil
}

// ExportFile exports a Google Workspace document in the given MIME type.
func (c *Client) ExportFile(id string, mimeType string) ([]byte, error) {
	params := url.Values{}
	params.Set("mimeType", mimeType)
	req, err := http.NewRequest(http.MethodGet, c.buildURL("/files/"+id+"/export", params), nil)
	if err != nil {
		return nil, err
	}
	data, _, err := c.doRawRequest(req)
	return data, err
}

func (c *Client) GetAbout() (*About, error) {
	params := url.Values{}
	params.Set("fields", "user,storageQuota")
	body, err := c.Get("/about", params)
	if err != nil {
		return nil, err
	}
	var a About
	return &a, json.Unmarshal(body, &a)
}

// ---- Drive (Shared Drive) methods ----

const driveFields = "id,name,kind,createdTime,capabilities"

func (c *Client) ListDrives(query string, pageToken string, pageSize int) ([]*Drive, string, error) {
	params := url.Values{}
	params.Set("fields", "nextPageToken,drives("+driveFields+")")
	params.Set("pageSize", strconv.Itoa(pageSize))
	if query != "" {
		params.Set("q", query)
	}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}
	body, err := c.Get("/drives", params)
	if err != nil {
		return nil, "", err
	}
	var resp struct {
		Drives        []*Drive `json:"drives"`
		NextPageToken string   `json:"nextPageToken"`
	}
	return resp.Drives, resp.NextPageToken, json.Unmarshal(body, &resp)
}

func (c *Client) GetDrive(id string) (*Drive, error) {
	params := url.Values{}
	params.Set("fields", driveFields)
	body, err := c.Get("/drives/"+id, params)
	if err != nil {
		return nil, err
	}
	var d Drive
	return &d, json.Unmarshal(body, &d)
}

func (c *Client) CreateDrive(name string, requestID string) (*Drive, error) {
	params := url.Values{}
	params.Set("requestId", requestID)
	params.Set("fields", driveFields)
	body, err := c.Post("/drives", params, map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	var d Drive
	return &d, json.Unmarshal(body, &d)
}

func (c *Client) DeleteDrive(id string) error {
	return c.Delete("/drives/"+id, nil)
}

// ---- Permission methods ----

const permFields = "id,type,role,emailAddress,displayName,expirationTime,deleted"

func (c *Client) ListPermissions(fileID string) ([]*Permission, error) {
	params := url.Values{}
	params.Set("fields", "permissions("+permFields+")")
	body, err := c.Get("/files/"+fileID+"/permissions", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Permissions []*Permission `json:"permissions"`
	}
	return resp.Permissions, json.Unmarshal(body, &resp)
}

func (c *Client) GetPermission(fileID, permID string) (*Permission, error) {
	params := url.Values{}
	params.Set("fields", permFields)
	body, err := c.Get("/files/"+fileID+"/permissions/"+permID, params)
	if err != nil {
		return nil, err
	}
	var p Permission
	return &p, json.Unmarshal(body, &p)
}

func (c *Client) CreatePermission(fileID string, perm map[string]string) (*Permission, error) {
	params := url.Values{}
	params.Set("fields", permFields)
	if perm["type"] == "domain" || perm["type"] == "anyone" {
		params.Set("sendNotificationEmail", "false")
	}
	body, err := c.Post("/files/"+fileID+"/permissions", params, perm)
	if err != nil {
		return nil, err
	}
	var p Permission
	return &p, json.Unmarshal(body, &p)
}

func (c *Client) DeletePermission(fileID, permID string) error {
	return c.Delete("/files/"+fileID+"/permissions/"+permID, nil)
}

// ---- Changes methods ----

func (c *Client) GetStartPageToken(driveID string) (string, error) {
	params := url.Values{}
	params.Set("fields", "startPageToken")
	if driveID != "" {
		params.Set("driveId", driveID)
		params.Set("supportsAllDrives", "true")
	}
	body, err := c.Get("/changes/startPageToken", params)
	if err != nil {
		return "", err
	}
	var resp struct {
		StartPageToken string `json:"startPageToken"`
	}
	return resp.StartPageToken, json.Unmarshal(body, &resp)
}

func (c *Client) ListChanges(pageToken string, pageSize int, driveID string) ([]*Change, string, string, error) {
	params := url.Values{}
	params.Set("pageToken", pageToken)
	params.Set("pageSize", strconv.Itoa(pageSize))
	params.Set("fields", "nextPageToken,newStartPageToken,changes(kind,changeType,time,removed,fileId,driveId,file(id,name,mimeType,modifiedTime,trashed))")
	if driveID != "" {
		params.Set("driveId", driveID)
		params.Set("supportsAllDrives", "true")
		params.Set("includeItemsFromAllDrives", "true")
	}
	body, err := c.Get("/changes", params)
	if err != nil {
		return nil, "", "", err
	}
	var resp struct {
		Changes           []*Change `json:"changes"`
		NextPageToken     string    `json:"nextPageToken"`
		NewStartPageToken string    `json:"newStartPageToken"`
	}
	return resp.Changes, resp.NextPageToken, resp.NewStartPageToken, json.Unmarshal(body, &resp)
}
