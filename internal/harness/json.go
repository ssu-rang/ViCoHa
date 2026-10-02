package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// StrictJSON rejects duplicate keys and trailing output instead of repairing it.
func StrictJSON(out string, dst any) error {
	d := json.NewDecoder(strings.NewReader(out))
	var value func() error
	value = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				k, ok := key.(string)
				if !ok || seen[k] {
					return fmt.Errorf("invalid or duplicate JSON key %v", key)
				}
				seen[k] = true
				if err := value(); err != nil {
					return err
				}
			}
			_, err = d.Token()
		case json.Delim('['):
			for d.More() {
				if err := value(); err != nil {
					return err
				}
			}
			_, err = d.Token()
		}
		return err
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return json.Unmarshal([]byte(out), dst)
}

func fields(object map[string]json.RawMessage, required []string, optional ...string) error {
	allowed := map[string]bool{}
	for _, k := range required {
		if object[k] == nil || string(object[k]) == "null" {
			return fmt.Errorf("missing or null field %s", k)
		}
		allowed[k] = true
	}
	for _, k := range optional {
		allowed[k] = true
	}
	for k := range object {
		if !allowed[k] {
			return fmt.Errorf("unknown field %s", k)
		}
	}
	return nil
}
