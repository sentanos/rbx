package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type getByUsernameAPIRequest struct {
	Usernames          []string `json:"usernames"`
	ExcludeBannedUsers bool     `json:"excludeBannedUsers"`
}

type getByUsernameAPIResponse struct {
	Data []struct {
		UserID int64 `json:"id"`
	} `json:"data"`
}

type getByUserIDAPIResponse struct {
	Username string `json:"name"`
}

func shortPoll(lastID int64, interval time.Duration, fun func(int64,
	chan<- interface{}) (int64, error), updates chan interface{},
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

func IDFromUsername(username string) (int64, error) {
	req := getByUsernameAPIRequest{
		Usernames:          []string{username},
		ExcludeBannedUsers: false,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return 0, errors.New("IDFromUsername marshal request")
	}
	httpRes, err := http.Post("https://users.roblox.com/v1/usernames/users", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return 0, errors.Wrap(err, "IDFromUsername request")
	}
	defer httpRes.Body.Close()

	apiResponse := &getByUsernameAPIResponse{}
	err = json.NewDecoder(httpRes.Body).Decode(apiResponse)
	if err != nil {
		return 0, errors.Wrap(err, "IDFromUsername decode")
	}
	if len(apiResponse.Data) == 0 {
		return 0, errors.New("UserNotFound")
	}
	return apiResponse.Data[0].UserID, nil
}

func UsernameFromID(userID string) (string, error) {
	httpRes, err := http.Get(fmt.Sprintf("https://users.roblox.com/v1/users/%s", userID))
	if err != nil {
		return "", errors.Wrap(err, "UsernameFromID request")
	}
	if httpRes.StatusCode == 404 {
		return "", errors.New("UserNotFound")
	}

	defer httpRes.Body.Close()
	apiResponse := &getByUserIDAPIResponse{}
	err = json.NewDecoder(httpRes.Body).Decode(apiResponse)
	if err != nil {
		return "", errors.Wrap(err, "UsernameFromID decode")
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
