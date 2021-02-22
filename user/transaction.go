package user

import (
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const RefreshSalesInterval = time.Second * 3

type RobloxAgent struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type RobloxItem struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type RobloxCurrency struct {
	Amount int    `json:"amount"`
	Type   string `json:"type"`
}

type Transaction struct {
	ID        int64          `json:"id"`
	Date      time.Time      `json:"created"`
	Agent     RobloxAgent    `json:"agent"`
	Item      RobloxItem     `json:"details"`
	Currency  RobloxCurrency `json:"currency"`
	IsPending bool           `json:"isPending"`
	Type      TransactionType
}

type transactionManager struct {
	lastID int64
}

func (transaction Transaction) String() string {
	date := transaction.Date.Format("2006-01-02 15:04:05")
	var direction string
	var transactionString string
	var pendString string
	if transaction.Type == Purchase {
		direction = "from"
		transactionString = "Purchased"
	} else {
		direction = "to"
		transactionString = "Sold"
	}
	if transaction.IsPending {
		pendString = "Pending"
	} else {
		pendString = ""
	}
	return fmt.Sprintf("%s [%d] %s %s %s (%d) %s %s %s (%d) for %d %s @ %s",
		pendString, transaction.ID, transactionString,
		strings.ToLower(transaction.Item.Type), transaction.Item.Name,
		transaction.Item.ID, direction, strings.ToLower(transaction.Agent.
			Type), transaction.Agent.Name, transaction.Agent.ID,
		transaction.Currency.Amount, strings.ToLower(transaction.Currency.
			Type), date)
}

type transactionsResponse struct {
	PreviousPageCursor *string       `json:"previousPageCursor"`
	NextPageCursor     *string       `json:"nextPageCursor"`
	Data               []Transaction `json:"data"`
}

type TransactionType int

const (
	Sale TransactionType = iota
	Purchase
)

func (user *User) GetTransactions(transactionType TransactionType,
	cursor *string) (
	[]Transaction, *string, error) {
	var transactionString string
	switch transactionType {
	case Sale:
		transactionString = "Sale"
	case Purchase:
		transactionString = "Purchase"
	}
	opt := url.Values{}
	opt.Set("transactionType", transactionString)
	opt.Set("limit", "10")
	if cursor != nil {
		opt.Set("cursor", *cursor)
	}
	res, err := user.Client.Get("https://economy.roblox.com/v2/users/" +
		strconv.FormatInt(user.UserID, 10) + "/transactions?" + opt.Encode())
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to retrieve transactions")
	}
	if res.StatusCode != 200 {
		return nil, nil, errors.Errorf("Unknown status code %d", res.StatusCode)
	}
	transResp := transactionsResponse{}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&transResp)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to parse JSON")
	}
	return transResp.Data, transResp.NextPageCursor, nil
}

func (user *User) syncSales(lastID int64, sales chan<- interface{}) (int64,
	error) {
	if lastID == -1 {
		transactions, _, err := user.GetTransactions(Sale, nil)
		if err != nil {
			return lastID, errors.Wrap(err, "Failed to retrieve transactions")
		}
		if len(transactions) > 0 {
			return transactions[0].ID, nil
		}
	} else {
		var next *string
		page := 0
		next = nil
		for {
			transactions, n, err := user.GetTransactions(Sale, next)
			next = n
			if err != nil {
				return lastID, errors.Wrapf(err,
					"Failed to retrieve page %d of transactions", page)
			}
			for i := 0; i < len(transactions) && transactions[i].ID > lastID; i++ {
				sales <- transactions[i]
			}
			if len(transactions) == 0 {
				break
			} else if transactions[len(transactions)-1].
				ID <= lastID || lastID == -1 {
				return transactions[0].ID, nil
			}
			page++
		}
	}
	return lastID, nil
}

func (user *User) TrackSales(lastID int64) (<-chan Transaction, <-chan error,
	chan<- bool) {
	updates := make(chan interface{}, 1)
	newSales := make(chan Transaction, 1)
	errorChan := make(chan error, 1)
	cancel := make(chan bool, 1)
	go shortPoll(lastID, RefreshSalesInterval, user.syncSales, updates,
		errorChan, cancel)
	go (func() {
		for {
			select {
			case update := <-updates:
				newSales <- update.(Transaction)
			case stop := <-cancel:
				if stop {
					return
				}
			}
		}
	})()
	return newSales, errorChan, cancel
}
