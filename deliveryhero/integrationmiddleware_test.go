// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package deliveryhero

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/oauth2/internal"
)

func newConf(serverURL string) *Config {
	return &Config{
		UserName: "UserName",
		Password: "Password",
		TokenURL: serverURL + "/v2/login",
	}
}

type mockTransport struct {
	rt func(req *http.Request) (resp *http.Response, err error)
}

func (t *mockTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	return t.rt(req)
}

func TestTokenSourceGrantTypeOverride(t *testing.T) {
	wantGrantType := "client_credentials"
	var gotGrantType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ioutil.ReadAll(r.Body) == %v, %v, want _, <nil>", body, err)
		}
		if err := r.Body.Close(); err != nil {
			t.Errorf("r.Body.Close() == %v, want <nil>", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("url.ParseQuery(%q) == %v, %v, want _, <nil>", body, values, err)
		}
		gotGrantType = values.Get("grant_type")
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.Write([]byte("access_token=90d64460d14870c08c81352a05dedd3465940a7c&token_type=bearer"))
	}))
	config := &Config{
		UserName: "UserName",
		Password: "Password",
		TokenURL: ts.URL + "/token",
	}
	token, err := config.TokenSource(context.Background()).Token()
	if err != nil {
		t.Errorf("config.TokenSource(_).Token() == %v, %v, want !<nil>, <nil>", token, err)
	}
	if gotGrantType != wantGrantType {
		t.Errorf("grant_type == %q, want %q", gotGrantType, wantGrantType)
	}
}

func TestTokenRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "/v2/login" {
			t.Errorf("authenticate client request URL = %q; want %q", r.URL, "/v2/login")
		}
		if got, want := r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"; got != want {
			t.Errorf("Content-Type header = %q; want %q", got, want)
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			r.Body.Close()
		}
		if err != nil {
			t.Errorf("failed reading request body: %s.", err)
		}
		if string(body) != "grant_type=client_credentials&password=Password&username=UserName" {
			t.Errorf("payload = %q; want %q", string(body), "grant_type=client_credentials&password=Password&username=UserName")
		}
		w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
		w.Write([]byte("access_token=90d64460d14870c08c81352a05dedd3465940a7c&token_type=bearer"))
	}))
	defer ts.Close()
	conf := newConf(ts.URL)
	tok, err := conf.Token(context.Background())
	if err != nil {
		t.Error(err)
	}
	if !tok.Valid() {
		t.Fatalf("token invalid. got: %#v", tok)
	}
	if tok.AccessToken != "90d64460d14870c08c81352a05dedd3465940a7c" {
		t.Errorf("Access token = %q; want %q", tok.AccessToken, "90d64460d14870c08c81352a05dedd3465940a7c")
	}
	if tok.TokenType != "bearer" {
		t.Errorf("token type = %q; want %q", tok.TokenType, "bearer")
	}
}

func TestTokenRefreshRequest(t *testing.T) {
	internal.ResetAuthCache()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() == "/somethingelse" {
			return
		}
		if r.URL.String() != "/v2/login" {
			t.Errorf("Unexpected token refresh request URL: %q", r.URL)
		}
		headerContentType := r.Header.Get("Content-Type")
		if got, want := headerContentType, "application/x-www-form-urlencoded"; got != want {
			t.Errorf("Content-Type = %q; want %q", got, want)
		}
		body, _ := ioutil.ReadAll(r.Body)
		const want = "grant_type=client_credentials&password=Password&username=UserName"
		if string(body) != want {
			t.Errorf("Unexpected refresh token payload.\n got: %s\nwant: %s\n", body, want)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"access_token": "foo"}`)
	}))
	defer ts.Close()
	conf := newConf(ts.URL)
	c := conf.Client(context.Background())
	c.Get(ts.URL + "/somethingelse")
}