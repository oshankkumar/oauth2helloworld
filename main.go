package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2/github"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	githubclient "github.com/google/go-github/v37/github"
	"github.com/pkg/browser"
	"golang.org/x/oauth2"
)

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Application struct {
	Doer   Doer
	Config *oauth2.Config
	Ctx    context.Context
}

func (a *Application) Oauth2Callback(w http.ResponseWriter, req *http.Request) {
	token, err := a.Config.Exchange(a.Ctx, req.FormValue("code"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("%+v\n", token)

	q := make(url.Values)
	q.Set("access_token", token.AccessToken)

	uri := &url.URL{
		Path:     "/welcome",
		RawQuery: q.Encode(),
	}
	http.Redirect(w, req, uri.String(), http.StatusFound)
}

func (a *Application) Welcome(w http.ResponseWriter, req *http.Request) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: req.FormValue("access_token")},
	)

	githubC := githubclient.NewClient(oauth2.NewClient(a.Ctx, ts))

	user, _, err := githubC.Users.Get(req.Context(), "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(user)
}

func (a *Application) Login(w http.ResponseWriter, req *http.Request) {
	qry := make(url.Values)
	qry.Set("client_id", a.Config.ClientID)
	qry.Set("redirect_uri", "http://localhost:8080/oauth2/callback")
	qry.Set("scope", strings.Join(a.Config.Scopes, ","))

	redirectURL := fmt.Sprintf("%s?%s", a.Config.Endpoint.AuthURL, qry.Encode())

	http.Redirect(w, req, redirectURL, http.StatusFound)
}

func main() {

	config := &oauth2.Config{
		ClientID:     os.Getenv("APPLICATION_CLIENT_ID"),
		ClientSecret: os.Getenv("APPLICATION_CLIENT_SECRET"),
		Endpoint:     github.Endpoint,
		RedirectURL:  "http://localhost:8080/oauth2/callback",
		Scopes:       []string{"user"},
	}

	helloWorld := &Application{
		Config: config,
		Ctx:    NewClientContext(),
	}

	http.Handle("/login", http.HandlerFunc(helloWorld.Login))
	http.Handle("/oauth2/callback", http.HandlerFunc(helloWorld.Oauth2Callback))
	http.Handle("/welcome", http.HandlerFunc(helloWorld.Welcome))
	http.Handle("/", http.FileServer(http.Dir("./web")))

	log.Println("started server: visit http://localhost:8080")

	defer time.AfterFunc(time.Second, func() {
		log.Println("opening URL http://localhost:8080")
		_ = browser.OpenURL("http://localhost:8080")
	}).Stop()

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalln("error starting server", err)
	}
}

func NewClientContext() context.Context {
	return context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: &wrappedRoundTripper{}})
}

type wrappedRoundTripper struct{}

func (w *wrappedRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := httputil.DumpRequestOut(request, true)
	if err != nil {
		log.Println("Error:", err)
		return http.DefaultClient.Do(request)
	}

	fmt.Println(string(body))
	return http.DefaultClient.Do(request)
}
