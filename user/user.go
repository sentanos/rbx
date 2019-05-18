package user

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/sentanos/rbx/client"
	"net/http"
)

type User struct {
	Client             *client.Client
	transactionManager transactionManager
}

func LoginWithCookie(cookie string) (*User, error) {
	userClient, err := client.FromCookie(cookie)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create client")
	}
	return &User{userClient, transactionManager{-1}}, nil
}

type userInfoResponse struct {
	UserID   int
	UserName string
}

// Returns user ID and username of the logged in user
func (user *User) Status() (int, string, error) {
	prev := user.Client.CheckRedirect
	user.Client.CheckRedirect = func(req *http.Request,
		via []*http.Request) error {
		return http.ErrUseLastResponse // Don't follow redirects
	}

	res, err := user.Client.Get("https://www.roblox.com/mobileapi/userinfo")
	if err != nil {
		return 0, "", errors.Wrap(err, "Failed to retrieve user info")
	}
	defer res.Body.Close()
	if res.StatusCode == 302 {
		return 0, "", errors.New("You are not logged in")
	}
	userInfo := &userInfoResponse{}
	err = json.NewDecoder(res.Body).Decode(userInfo)
	if err != nil {
		return 0, "", errors.Wrap(err, "Failed to decode json")
	}

	user.Client.CheckRedirect = prev
	return userInfo.UserID, userInfo.UserName, nil
}

func (user *User) Relog() error {
	req, err := user.Client.NewVerifiedRequest("https://www.roblox."+
		"com/authentication/signoutfromallsessionsandreauthenticate",
		"https://www.roblox.com/my/account#!/security")
	if err != nil {
		return errors.Wrap(err, "Failed to create verified request")
	}
	res, err := user.Client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Request failed")
	}
	if res.StatusCode != 200 {
		return errors.Errorf("Unknown status code %d", res.StatusCode)
	}
	return nil
}
