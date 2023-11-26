package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type TranslateOptions struct {
	From         string
	To           string
	Host         string
	FetchOptions map[string]interface{}
}

type RawResponse struct {
	Sentences []Sentence `json:"sentences"`
}

// Sentence represents a translated sentence
type Sentence struct {
	Trans string `json:"trans"`
}

type Translator struct {
	InputText string
	Options   TranslateOptions
}

var DefaultOptions TranslateOptions

func init() {
	DefaultOptions = TranslateOptions{
		From: "auto",
		To:   "vi",
		Host: "translate.google.com",
		FetchOptions: map[string]interface{}{
			"method": "POST",
			"headers": map[string]string{
				"Content-Type": "application/x-www-form-urlencoded;charset=utf-8",
			},
		},
	}
}

func NewTranslator(inputText string, options TranslateOptions) *Translator {
	defaults := TranslateOptions{
		From: "auto",
		To:   "vi",
		Host: "translate.google.com",
		FetchOptions: map[string]interface{}{
			"method": "POST",
			"headers": map[string]string{
				"Content-Type": "application/x-www-form-urlencoded;charset=utf-8",
			},
		},
	}

	options.From = defaults.From
	options.To = defaults.To
	options.Host = defaults.Host

	// Merge fetch options
	for key, value := range defaults.FetchOptions {
		if _, exists := options.FetchOptions[key]; !exists {
			options.FetchOptions[key] = value
		}
	}

	return &Translator{
		InputText: inputText,
		Options:   options,
	}
}

func (t *Translator) Translate(ctx context.Context) (*RawResponse, error) {
	url := t.buildURL()
	body := t.buildBody()

	req, err := http.NewRequestWithContext(ctx, t.Options.FetchOptions["method"].(string), url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	// Set request headers
	headers := t.Options.FetchOptions["headers"].(map[string]string)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", res.Status)
	}

	raw, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var response RawResponse
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func ToSentence(ctx context.Context, input string) (string, error) {
	instance := NewTranslator(input, DefaultOptions)
	response, err := instance.Translate(ctx)
	if err != nil {
		return "", err
	}
	result := ""
	for _, sentence := range response.Sentences {
		result += sentence.Trans
	}
	return result, nil
}

func (t *Translator) buildURL() string {
	host := t.Options.Host
	return fmt.Sprintf("https://%s/translate_a/single?client=at&dt=t&dt=rm&dj=1", host)
}

func (t *Translator) buildBody() string {
	params := url.Values{
		"sl": {t.Options.From},
		"tl": {t.Options.To},
		"q":  {t.InputText},
	}
	return params.Encode()
}
