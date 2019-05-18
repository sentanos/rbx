package client

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

const XSRF = "X-CSRF-TOKEN"
const MaxDepth = 3
const BaseURL = "https://www.roblox.com"

var VerificationInputs = [...]string{
	"__RequestVerificationToken", // Leave this as first entry!
}

type Client struct {
	inner         *http.Client
	CheckRedirect func(*http.Request, []*http.Request) error
}

func (client *Client) SetupRedirectHandler() {
	client.inner.CheckRedirect = func(req *http.Request,
		via []*http.Request) error {
		if client.CheckRedirect != nil {
			return client.CheckRedirect(req, via)
		} else {
			return nil
		}
	}
}

func FromCookie(cookie string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create cookie jar")
	}
	URL, err := url.Parse(BaseURL)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse base URL")
	}
	cookies := make([]*http.Cookie, 1)
	cookies[0] = &http.Cookie{
		Name:     ".ROBLOSECURITY",
		Value:    cookie,
		Domain:   ".roblox.com",
		HttpOnly: true,
		MaxAge:   0,
	}
	jar.SetCookies(URL, cookies)
	newClient := &Client{inner: &http.Client{Jar: jar}}
	newClient.SetupRedirectHandler()
	return newClient, nil
}

func (client *Client) handle(req *http.Request, depth int) (*http.Response, error) {
	// If there is a body it will be cloned in case the request needs to be
	// re-executed
	hasBody := req.Body != nil
	var clone io.ReadCloser
	var err error
	if hasBody {
		clone, err = req.GetBody()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to clone body")
		}
	}
	res, err := client.inner.Do(req)
	if err != nil {
		return res, err
	}
	if res.StatusCode == 403 &&
		(res.Status == "403 XSRF Token Validation Failed" ||
			res.Status == "403 Token Validation Failed") {
		token := res.Header.Get(XSRF)
		if token == "" {
			if depth >= MaxDepth {
				return nil, errors.Errorf(
					"Failed to retrieve %s after %d tries", XSRF, depth)
			} else {
				return nil, errors.Errorf("Failed to retrieve %s", XSRF)
			}
		}
		req.Header.Add(XSRF, token)
		if hasBody {
			req.Body = clone
		}
		return client.handle(req, depth+1)
	} else {
		return res, nil
	}
}

func getVerificationInputs(res *http.Response) (*url.Values, error) {
	defer res.Body.Close()
	inputs := &url.Values{}
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse body")
	}
	for _, input := range VerificationInputs {
		selection := doc.Find(fmt.Sprintf("input[name=\"%s\"]", input))
		if selection.Length() > 0 {
			val, ok := selection.Attr("value")
			if ok {
				inputs.Add(input, val)
			}
		}
	}
	return inputs, nil
}

// Creates a request with verification headers and inputs populated.
// From is where verification is retrieved. If from is an empty string,
// it will be assumed that the source of verification is the same as the
// request url.
func (client *Client) NewVerifiedRequest(url string,
	from string) (*http.Request, error) {
	if from == "" {
		from = url
	}
	initial, err := http.NewRequest("GET", from, nil)
	if err != nil {
		return nil, errors.Wrap(err,
			"Failed to create initial verification retrieval request")
	}
	res, err := client.handle(initial, 0)
	if err != nil {
		return nil, errors.Wrap(err,
			"Initial verification retrieval request failed")
	}
	form, err := getVerificationInputs(res)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get verification inputs")
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create request")
	}
	header := res.Header.Get(VerificationInputs[0])
	if header != "" {
		req.Header.Set(VerificationInputs[0], header)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func (client *Client) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create request")
	}
	return client.Do(req)
}

func (client *Client) Do(req *http.Request) (*http.Response, error) {
	return client.handle(req, 0)
}

func (client *Client) Post(url, contentType string,
	body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create request")
	}
	req.Header.Set("Content-Type", contentType)
	return client.Do(req)
}

func (client *Client) PostForm(url string, data url.Values) (*http.Response,
	error) {
	return client.Post(url, "application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()))
}
