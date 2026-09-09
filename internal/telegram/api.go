package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.telegram.org"

type Update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

type apiResp struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

var httpClient = &http.Client{Timeout: 20 * time.Second}

func GetMe(token string) (username string, err error) {
	var me struct {
		Username string `json:"username"`
	}
	if err := call(token, "getMe", nil, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}

func GetUpdates(token string, offset int64) ([]Update, error) {
	q := url.Values{}
	if offset > 0 {
		q.Set("offset", strconv.FormatInt(offset, 10))
	}
	q.Set("timeout", "0")
	q.Set("limit", "20")
	var updates []Update
	if err := call(token, "getUpdates", q, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func SendMessage(token, chatID, text string) error {
	q := url.Values{}
	q.Set("chat_id", chatID)
	q.Set("text", text)
	return call(token, "sendMessage", q, nil)
}

func ChatIDString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func call(token, method string, q url.Values, dest any) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	u := apiBase + "/bot" + token + "/" + method
	if q != nil {
		u += "?" + q.Encode()
	}
	res, err := httpClient.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	var wrap apiResp
	if err := json.Unmarshal(body, &wrap); err != nil {
		return fmt.Errorf("telegram: %s", strings.TrimSpace(string(body)))
	}
	if !wrap.OK {
		if wrap.Description != "" {
			return fmt.Errorf("telegram: %s", wrap.Description)
		}
		return fmt.Errorf("telegram api error")
	}
	if dest == nil || len(wrap.Result) == 0 {
		return nil
	}
	return json.Unmarshal(wrap.Result, dest)
}
