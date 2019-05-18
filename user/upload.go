package user

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type ModelOptions struct {
	Name          string
	Description   string
	CopyLocked    bool
	AllowComments bool
	GroupID       int
}

func (user *User) UploadModel(body io.Reader, options ModelOptions) (string,
	string, error) {
	parseOpt := url.Values{}
	parseOpt.Set("type", "Model")
	parseOpt.Set("genreTypeId", "1")
	parseOpt.Set("json", "1")
	parseOpt.Set("assetid", "0")

	parseOpt.Set("name", options.Name)
	parseOpt.Set("description", options.Description)
	parseOpt.Set("ispublic", strconv.FormatBool(!options.CopyLocked))
	parseOpt.Set("allowComments", strconv.FormatBool(options.AllowComments))
	if options.GroupID != -1 {
		parseOpt.Set("groupId", string(options.GroupID))
	}

	req, err := http.NewRequest("POST", "https://data.roblox.com/Data/Upload."+
		"ashx?"+parseOpt.Encode(), body)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to create request")
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("User-Agent", "") // For some reason this is very important
	res, err := user.Client.Do(req)
	if err != nil {
		return "", "", errors.Wrap(err, "Request failed")
	}
	if res.StatusCode != 200 {
		return "", "", errors.Errorf("Upload failed: Unknown status code %d",
			res.StatusCode)
	}

	var data map[string]int
	err = json.NewDecoder(res.Body).Decode(&data)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to parse JSON")
	}
	asset, ok := data["AssetId"]
	if !ok {
		return "", "", errors.New("Invalid JSON")
	}
	assetVersion, ok := data["AssetVersionId"]
	if !ok {
		return "", "", errors.New("Invalid JSON")
	}
	return strconv.Itoa(asset), strconv.Itoa(assetVersion), nil
}
