package user

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const RefreshSalesInterval = time.Second * 3

type RobloxUser struct {
	ID   int
	Name string
}

type RobloxItem struct {
	ID   int
	Name string
}

type Transaction struct {
	ID     int
	Date   time.Time
	User   RobloxUser
	Item   RobloxItem
	Amount int
	Type   TransactionType
}

type transactionManager struct {
	lastID int
}

func (transaction Transaction) String() string {
	date := transaction.Date.Format("2006-01-02")
	var direction string
	var transactionString string
	if transaction.Type == Purchase {
		direction = "from"
		transactionString = "Purchased"
	} else {
		direction = "to"
		transactionString = "Sold"
	}
	return fmt.Sprintf("[%d] %s item %s (%d) %s %s (%d) for %d robux @ %s",
		transaction.ID, transactionString, transaction.Item.Name,
		transaction.Item.ID, direction, transaction.User.Name,
		transaction.User.ID, transaction.Amount, date)
}

type transactionsResponse struct {
	Data []string
}

type transactionResponse struct {
	Amount   string
	Date     string
	ItemName string `json:"Item_Name"`
	ItemUrl  string `json:"Item_Url"`
	Member   string
	MemberID string `json:"Member_ID"`
	SaleID   int    `json:"Sale_ID"`
}

type TransactionType int

const (
	Sale TransactionType = iota
	Purchase
)

func (user *User) GetTransactions(start int, transactionType TransactionType) (
	[]Transaction, error) {
	var transactionString string
	switch transactionType {
	case Sale:
		transactionString = "sale"
	case Purchase:
		transactionString = "purchase"
	}
	form := map[string]string{
		"transactiontype": transactionString,
		"startindex":      string(start),
	}
	marshaled, err := json.Marshal(form)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to marshal json")
	}
	res, err := user.Client.Post("https://www.roblox.com/My/money."+
		"aspx/getmytransactions", "application/json",
		bytes.NewReader(marshaled))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to retrieve transactions")
	}
	if res.StatusCode != 200 {
		return nil, errors.Errorf("Unknown status code %d", res.StatusCode)
	}
	dataContainer := make(map[string]string)
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&dataContainer)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse JSON")
	}
	data, ok := dataContainer["d"]
	if !ok {
		return nil, errors.New("No data found")
	}
	holder := &transactionsResponse{}
	err = json.NewDecoder(strings.NewReader(data)).Decode(&holder)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse data JSON")
	}

	re := regexp.MustCompile(`https://www\.roblox\.com/library/(\d+)/`)
	var transactions []Transaction
	for _, row := range holder.Data {
		rowContainer := transactionResponse{}
		err = json.NewDecoder(strings.NewReader(row)).Decode(&rowContainer)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse row")
		}
		transaction := Transaction{
			User: RobloxUser{
				Name: rowContainer.Member,
			},
			Item: RobloxItem{
				Name: rowContainer.ItemName,
			},
		}
		transaction.User.ID, err = strconv.Atoi(rowContainer.MemberID)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse user ID")
		}
		matches := re.FindStringSubmatch(rowContainer.ItemUrl)
		if len(matches) != 2 {
			return nil, errors.Errorf(
				"regex match failed: unknown number of matches %d",
				len(matches))
		}
		transaction.Item.ID, err = strconv.Atoi(matches[1])
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse item ID")
		}
		amount, err := strconv.Atoi(rowContainer.Amount)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse amount")
		}
		if transactionType == Purchase {
			transaction.Amount = amount * -1
		} else {
			transaction.Amount = amount
		}
		date, err := time.Parse("1/2/06", rowContainer.Date)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to parse date")
		}
		transaction.Date = date
		transaction.Type = transactionType
		transaction.ID = rowContainer.SaleID
		transactions = append(transactions, transaction)
	}
	return transactions, nil
}

func (user *User) syncSales(lastID int, sales chan<- interface{}) (int, error) {
	if lastID == -1 {
		transactions, err := user.GetTransactions(0, Sale)
		if err != nil {
			return lastID, errors.Wrap(err, "Failed to retrieve transactions")
		}
		if len(transactions) > 0 {
			return transactions[0].ID, nil
		}
	} else {
		page := 0
		for {
			transactions, err := user.GetTransactions(page*20, Sale)
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

func (user *User) TrackSales(lastID int) (<-chan Transaction, <-chan error,
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
