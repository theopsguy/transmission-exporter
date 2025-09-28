package transmission

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testFixture struct {
	server *httptest.Server
	client *Client
}

func newTestFixture(t *testing.T, handler http.Handler) *testFixture {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := New(server.URL, nil)

	return &testFixture{
		server: server,
		client: client,
	}
}

func createHTTPHandler(statusCode int, headers map[string]string, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for key, value := range headers {
			w.Header().Set(key, value)
		}

		w.WriteHeader(statusCode)

		if len(body) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}
	}
}

func TestNew(t *testing.T) {
	user := &User{Username: "test", Password: "pass"}
	client := New("http://example.com", user)

	assert.Equal(t, "http://example.com/transmission/rpc/", client.URL)
	assert.Equal(t, user, client.User)
}

func TestGetToken(t *testing.T) {
	testCases := []struct {
		name          string
		handler       http.HandlerFunc
		expectedToken string
		expectError   bool
		errorContains string
	}{
		{
			name: "Success",
			handler: createHTTPHandler(
				http.StatusConflict,
				map[string]string{"X-Transmission-Session-Id": "test-token-123"},
				[]byte{},
			),
			expectedToken: "test-token-123",
			expectError:   false,
		},
		{
			name: "ServerError",
			handler: createHTTPHandler(
				http.StatusInternalServerError,
				nil,
				[]byte("Internal Server Error"),
			),
			expectedToken: "",
			expectError:   true,
			errorContains: "status code: 500",
		},
		{
			name: "BadResponse",
			handler: createHTTPHandler(
				http.StatusOK,
				nil,
				[]byte{},
			),
			expectedToken: "",
			expectError:   true,
			errorContains: "unexpected response trying to obtain token, status code: 200",
		},
		{
			name: "AuthFailure",
			handler: createHTTPHandler(
				http.StatusUnauthorized,
				nil,
				[]byte{},
			),
			expectedToken: "",
			expectError:   true,
			errorContains: "authorization failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newTestFixture(t, tc.handler)
			err := fixture.client.getToken()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedToken, fixture.client.token)
		})
	}
}

func TestAuthRequest(t *testing.T) {
	testCases := []struct {
		name           string
		initialToken   string
		expectedToken  string
		getTokenError  bool
		method         string
		expectedMethod string
		handler        http.HandlerFunc
		body           []byte
		expectError    bool
	}{
		{
			name:           "With existing token",
			initialToken:   "existing-token-123",
			expectedToken:  "existing-token-123",
			method:         "POST",
			expectedMethod: "POST",
			handler: createHTTPHandler(
				http.StatusConflict,
				map[string]string{"X-Transmission-Session-Id": "new-token-456"},
				nil,
			),
			body:        []byte(`{"test":"data"}`),
			expectError: false,
		},
		{
			name:           "No token but successful getToken",
			initialToken:   "",
			expectedToken:  "new-token-456",
			method:         "GET",
			expectedMethod: "GET",
			handler: createHTTPHandler(
				http.StatusConflict,
				map[string]string{"X-Transmission-Session-Id": "new-token-456"},
				nil,
			),
			body:        []byte(``),
			expectError: false,
		},
		{
			name:          "No token and getToken fails",
			initialToken:  "",
			expectedToken: "",
			getTokenError: true,
			method:        "POST",
			handler: createHTTPHandler(
				http.StatusInternalServerError,
				nil,
				[]byte("Internal Server Error"),
			),
			body:        []byte(`{"test":"data"}`),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newTestFixture(t, tc.handler)
			client := fixture.client
			client.token = tc.initialToken
			client.User = &User{Username: "testuser", Password: "testpass"}

			req, err := client.authRequest(tc.method, tc.body)
			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, req)
			assert.Equal(t, tc.expectedMethod, req.Method)

			expectedURL := fixture.server.URL + endpoint
			assert.Equal(t, expectedURL, req.URL.String())
			assert.Equal(t, tc.expectedToken, req.Header.Get("X-Transmission-Session-Id"))

			username, password, ok := req.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "testuser", username)
			assert.Equal(t, "testpass", password)
		})
	}
}
func TestPost(t *testing.T) {
	testCases := []struct {
		name          string
		handlers      []http.HandlerFunc
		requestBody   []byte
		initialToken  string
		expectedResp  []byte
		expectError   bool
		errorContains string
	}{
		{
			name: "Success with existing token",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusOK,
					map[string]string{"X-Transmission-Session-Id": "test-token-123"},
					[]byte(`{"result":"success"}`),
				),
			},
			initialToken: "test-token-123",
			requestBody:  []byte(`{"method":"test"}`),
			expectedResp: []byte(`{"result":"success"}`),
			expectError:  false,
		},
		{
			name: "Token refresh required",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusConflict,
					map[string]string{"X-Transmission-Session-Id": "new-token-456"},
					[]byte{},
				),
				createHTTPHandler(
					http.StatusOK,
					map[string]string{"X-Transmission-Session-Id": "new-token-456"},
					[]byte(`{"result":"success"}`),
				),
			},
			requestBody:  []byte(`{"method":"test"}`),
			expectedResp: []byte(`{"result":"success"}`),
			expectError:  false,
		},
		{
			name: "Unauthorized error",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusUnauthorized,
					nil,
					[]byte{},
				),
			},
			initialToken:  "test-token-123",
			requestBody:   []byte(`{"method":"test"}`),
			expectError:   true,
			errorContains: "request failed: authentication error",
		},
		{
			name: "Empty response body",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusOK,
					map[string]string{"X-Transmission-Session-Id": "test-token-123"},
					[]byte{},
				),
			},
			initialToken: "test-token-123",
			requestBody:  []byte(`{"method":"test"}`),
			expectedResp: []byte{},
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerIndex := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if handlerIndex < len(tc.handlers) {
					tc.handlers[handlerIndex](w, r)
					handlerIndex++
				}
			})

			fixture := newTestFixture(t, handler)
			client := fixture.client

			if tc.initialToken != "" {
				client.token = tc.initialToken
			}

			client.User = &User{Username: "testuser", Password: "testpass"}

			resp, err := client.post(tc.requestBody)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResp, resp)
		})
	}
}

func TestGetTorrents(t *testing.T) {
	testCases := []struct {
		name          string
		handlers      []http.HandlerFunc
		initialToken  string
		expectedResp  []Torrent
		expectError   bool
		errorContains string
	}{
		{
			name: "Success",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusOK,
					map[string]string{"X-Transmission-Session-Id": "test-token-123"},
					[]byte(`{
                        "arguments": {
                            "torrents": [
                                {
                                    "id": 1,
                                    "name": "Test Torrent",
                                    "status": 4,
                                    "totalSize": 100000,
                                    "percentDone": 0.5,
                                    "uploadRatio": 1.5,
                                    "rateDownload": 1000,
                                    "rateUpload": 500
                                }
                            ]
                        },
                        "result": "success"
                    }`),
				),
			},
			initialToken: "test-token-123",
			expectedResp: []Torrent{
				{
					ID:           1,
					Name:         "Test Torrent",
					Status:       4,
					TotalSize:    100000,
					PercentDone:  0.5,
					UploadRatio:  1.5,
					RateDownload: 1000,
					RateUpload:   500,
				},
			},
			expectError: false,
		},
		{
			name: "Empty response",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusOK,
					map[string]string{"X-Transmission-Session-Id": "test-token-123"},
					[]byte(`{"arguments":{"torrents":[]},"result":"success"}`),
				),
			},
			initialToken: "test-token-123",
			expectedResp: []Torrent{},
			expectError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			handlerIndex := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)

				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var cmd TorrentCommand
				err = json.Unmarshal(body, &cmd)
				assert.NoError(t, err)
				assert.Equal(t, "torrent-get", cmd.Method)

				if handlerIndex < len(tc.handlers) {
					tc.handlers[handlerIndex](w, r)
					handlerIndex++
				}
			})

			fixture := newTestFixture(t, handler)
			client := fixture.client
			client.token = tc.initialToken

			torrents, err := client.GetTorrents()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResp, torrents)
		})
	}
}

func TestGetSession(t *testing.T) {
	testCases := []struct {
		name          string
		handlers      []http.HandlerFunc
		initialToken  string
		expectedResp  *Session
		expectError   bool
		errorContains string
	}{
		{
			name: "Success",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusOK,
					map[string]string{"X-Transmission-Session-Id": "test-token-123"},
					[]byte(`{
                        "arguments": {
                            "alt-speed-down": 50,
                            "alt-speed-enabled": false,
                            "alt-speed-up": 50,
                            "download-dir": "/downloads",
                            "peer-limit-global": 200,
                            "peer-limit-per-torrent": 50,
                            "speed-limit-down": 100,
                            "speed-limit-down-enabled": true,
                            "speed-limit-up": 100,
                            "speed-limit-up-enabled": true,
                            "version": "4.0.3"
                        },
                        "result": "success"
                    }`),
				),
			},
			initialToken: "test-token-123",
			expectedResp: &Session{
				AltSpeedDown:          50,
				AltSpeedEnabled:       false,
				AltSpeedUp:            50,
				DownloadDir:           "/downloads",
				PeerLimitGlobal:       200,
				PeerLimitPerTorrent:   50,
				SpeedLimitDown:        100,
				SpeedLimitDownEnabled: true,
				SpeedLimitUp:          100,
				SpeedLimitUpEnabled:   true,
				Version:               "4.0.3",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var handlerIndex int
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)

				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var cmd SessionCommand
				err = json.Unmarshal(body, &cmd)
				assert.NoError(t, err)
				assert.Equal(t, "session-get", cmd.Method)

				if handlerIndex < len(tc.handlers) {
					tc.handlers[handlerIndex](w, r)
					handlerIndex++
				}
			})

			fixture := newTestFixture(t, handler)
			client := fixture.client
			client.token = tc.initialToken

			session, err := client.GetSession()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResp, session)
		})
	}
}

func TestGetSessionStats(t *testing.T) {
	testCases := []struct {
		name          string
		handlers      []http.HandlerFunc
		initialToken  string
		expectedResp  *SessionStats
		expectError   bool
		errorContains string
	}{
		{
			name: "Success",
			handlers: []http.HandlerFunc{
				createHTTPHandler(
					http.StatusOK,
					map[string]string{"X-Transmission-Session-Id": "test-token-123"},
					[]byte(`{
						"arguments": {
							"activeTorrentCount": 2,
							"downloadSpeed": 1000,
							"uploadSpeed": 500,
							"torrentCount": 5,
							"pausedTorrentCount": 3,
							"current-stats": {
								"uploadedBytes": 1000000,
								"downloadedBytes": 2000000,
								"filesAdded": 10,
								"secondsActive": 3600
							},
							"cumulative-stats": {
								"uploadedBytes": 5000000,
								"downloadedBytes": 10000000,
								"filesAdded": 50,
								"secondsActive": 86400
							}
						},
                        "result": "success"
                    }`),
				),
			},
			initialToken: "test-token-123",
			expectedResp: &SessionStats{
				ActiveTorrentCount: 2,
				DownloadSpeed:      1000,
				UploadSpeed:        500,
				TorrentCount:       5,
				PausedTorrentCount: 3,
				CurrentStats: SessionStateStats{
					UploadedBytes:   1000000,
					DownloadedBytes: 2000000,
					FilesAdded:      10,
					SecondsActive:   3600,
				},
				CumulativeStats: SessionStateStats{
					UploadedBytes:   5000000,
					DownloadedBytes: 10000000,
					FilesAdded:      50,
					SecondsActive:   86400,
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var handlerIndex int
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)

				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)

				var cmd SessionCommand
				err = json.Unmarshal(body, &cmd)
				assert.NoError(t, err)
				assert.Equal(t, "session-stats", cmd.Method)

				if handlerIndex < len(tc.handlers) {
					tc.handlers[handlerIndex](w, r)
					handlerIndex++
				}
			})

			fixture := newTestFixture(t, handler)
			client := fixture.client
			client.token = tc.initialToken

			sessionStats, err := client.GetSessionStats()

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResp, sessionStats)
		})
	}
}
