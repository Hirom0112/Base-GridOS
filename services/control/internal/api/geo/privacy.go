package geo

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

func ValidateResponse(data []byte) error {
	if !json.Valid(data) {
		return errors.New("invalid geo response JSON")
	}
	return scanGeoValue(json.NewDecoder(bytes.NewReader(data)))
}

func scanGeoValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid geo response field")
			}
			name := strings.ToLower(key)
			if strings.Contains(name, "address") || strings.Contains(name, "street") || strings.Contains(name, "house_number") || strings.Contains(name, "member_home") || strings.Contains(name, "home_location") {
				return errors.New("geo response contains a private location field")
			}
			if err := scanGeoValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case json.Delim('['):
		for decoder.More() {
			if err := scanGeoValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	}
	return nil
}
