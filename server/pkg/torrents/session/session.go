package session

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"

	"golang.org/x/net/publicsuffix"
)

type Credentials struct {
	Username string
	Password string
}

type Options struct {
	Name              string
	Origin            string
	LoginURL          string
	LoginValues       func(Credentials) url.Values
	IsLoginSuccessful func(*http.Response, *url.URL) bool
	IsLoginRedirect   func(*url.URL) bool
}

type AuthenticatedSession struct {
	options Options

	mu             sync.Mutex
	client         *http.Client
	credentials    Credentials
	hasCredentials bool
	authenticated  bool
	generation     uint64
}

func NewAuthenticatedSession(options Options) *AuthenticatedSession {
	return &AuthenticatedSession{options: options}
}

func (session *AuthenticatedSession) Fetch(requestURL string, credentials Credentials) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		client, generation, err := session.login(credentials)
		if err != nil {
			return nil, err
		}

		request, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}

		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}

		redirectURL := session.getRedirectURL(response)
		if !session.options.IsLoginRedirect(redirectURL) {
			return response, nil
		}

		drainAndClose(response)
		session.invalidate(generation)
	}

	return nil, fmt.Errorf("%s session was rejected after login", session.options.Name)
}

func (session *AuthenticatedSession) login(credentials Credentials) (*http.Client, uint64, error) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if !session.hasCredentials || session.credentials != credentials {
		if err := session.prepare(credentials); err != nil {
			return nil, 0, err
		}
	}

	if session.authenticated {
		return session.client, session.generation, nil
	}

	request, err := session.loginRequest(credentials)
	if err != nil {
		return nil, 0, err
	}

	response, err := session.client.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer drainAndClose(response)

	redirectURL := session.getRedirectURL(response)
	if !session.options.IsLoginSuccessful(response, redirectURL) {
		return nil, 0, fmt.Errorf(
			"%s login rejected (%s)",
			session.options.Name,
			response.Status,
		)
	}

	session.authenticated = true
	return session.client, session.generation, nil
}

func (session *AuthenticatedSession) loginRequest(credentials Credentials) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for name, values := range session.options.LoginValues(credentials) {
		for _, value := range values {
			if err := writer.WriteField(name, value); err != nil {
				return nil, err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, session.options.LoginURL, &body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	return request, nil
}

func (session *AuthenticatedSession) prepare(credentials Credentials) error {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return err
	}

	session.client = &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	session.credentials = credentials
	session.hasCredentials = true
	session.authenticated = false
	session.generation++

	return nil
}

func (session *AuthenticatedSession) invalidate(generation uint64) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if generation != session.generation {
		return
	}

	session.authenticated = false
	session.generation++
}

func (session *AuthenticatedSession) getRedirectURL(response *http.Response) *url.URL {
	if response.StatusCode != http.StatusFound {
		return nil
	}

	location := response.Header.Get("Location")
	if location == "" {
		return nil
	}

	redirectURL, err := url.Parse(location)
	if err != nil {
		return nil
	}

	origin, err := url.Parse(session.options.Origin)
	if err != nil {
		return nil
	}

	return origin.ResolveReference(redirectURL)
}

func drainAndClose(response *http.Response) {
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}
