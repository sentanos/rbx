package user

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type getByUsernameAPIResponse struct {
	ID           int    `json:"Id"`
	Username     string `json:"Username"`
	ErrorMessage string `json:"errorMessage"`
	Message      string `json:"message"`
}

type getByUserIDAPIErrors struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type getByUserIDAPIResponse struct {
	Username string                 `json:"Username"`
	Errors   []getByUserIDAPIErrors `json:"errors"`
}

func shortPoll(lastID int, interval time.Duration, fun func(int,
	chan<- interface{}) (int, error), updates chan interface{},
	errorChan chan<- error, cancel <-chan bool) {
	var err error
	ticker := time.NewTicker(interval)
	for {
		select {
		case stop := <-cancel:
			if stop {
				return
			}
		case <-ticker.C:
			lastID, err = fun(lastID, updates)
			if err != nil {
				errorChan <- err
			}
		}
	}
}

func IDFromUsername(username string) (int, error) {
	httpRes, err := http.Get(fmt.Sprintf("https://api.roblox.com/users/get-by-username?username=%s", url.QueryEscape(username)))
	if err != nil {
		return 0, errors.Wrap(err, "Request failed")
	}
	defer httpRes.Body.Close()

	apiResponse := &getByUsernameAPIResponse{}
	err = json.NewDecoder(httpRes.Body).Decode(apiResponse)
	if err != nil {
		return 0, errors.Wrap(err, "Parse JSON failed")
	}
	if apiResponse.ErrorMessage != "" || apiResponse.Message != "" {
		if apiResponse.ErrorMessage == "User not found" {
			return 0, errors.New("UserNotFound")
		} else {
			var message string
			if apiResponse.ErrorMessage == "" {
				message = apiResponse.Message
			} else {
				message = apiResponse.ErrorMessage
			}
			return 0, errors.New(message)
		}
	}
	return apiResponse.ID, nil
}

func UsernameFromID(userID string) (string, error) {
	httpRes, err := http.Get(fmt.Sprintf("https://api.roblox.com/users/%s", userID))
	if err != nil {
		return "", errors.Wrap(err, "Request failed")
	}
	defer httpRes.Body.Close()

	apiResponse := &getByUserIDAPIResponse{}
	err = json.NewDecoder(httpRes.Body).Decode(apiResponse)
	if err != nil {
		return "", errors.Wrap(err, "Parse JSON failed")
	}
	if len(apiResponse.Errors) > 0 {
		fullMessage := ""
		for i := 0; i < len(apiResponse.Errors); i++ {
			apiErr := apiResponse.Errors[i]
			if apiErr.Code == 400 {
				return "", errors.New("UserNotFound")
			}
			fullMessage += apiErr.Message
			fullMessage += ";"
		}
		return "", errors.New(fullMessage)
	}
	return apiResponse.Username, nil
}

func GetBlurb(userID string) (string, error) {
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	httpRes, err := client.Get(fmt.Sprintf("https://www.roblox."+
		"com/users/%s/profile", userID))
	if err != nil {
		return "", errors.Wrap(err, "Request failed")
	}
	defer httpRes.Body.Close()
	if httpRes.StatusCode != 200 {
		return "", errors.New("User does not exist")
	}

	doc, err := goquery.NewDocumentFromReader(httpRes.Body)
	if err != nil {
		return "", errors.Wrap(err, "Parse failed")
	}
	selection := doc.Find(".profile-about-content-text")
	if selection.Length() == 0 {
		return "", errors.New("Could not find blurb")
	}
	return selection.Eq(0).Text(), nil
}

func HasAsset(userID string, assetID string) (bool, error) {
	res, err := http.Get(fmt.Sprintf("https://api.roblox."+
		"com/ownership/hasasset?userId=%s&assetId=%s", userID, assetID))
	if err != nil {
		return false, errors.Wrap(err, "Request failed")
	}
	if res.StatusCode != 200 {
		return false, errors.Errorf("Unknown status code %d", res.StatusCode)
	}
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, errors.Wrap(err, "Read failed")
	}
	has, err := strconv.ParseBool(string(b))
	if err != nil {
		return false, errors.Wrap(err, "Parse failed")
	}
	return has, nil
}
