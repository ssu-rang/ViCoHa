package harness

import (
	"strings"
	"testing"
)

func TestDiscoveryContract(t *testing.T) {
	valid := `{"commands":[{"argv":["go","test","./..."],"reason":"existing Go tests","evidence":["go.mod"]}]}`
	for _, invalid := range []string{
		`null`, `{}`, `{"commands":null}`, `{"commands":[null]}`, valid + " trailing", "```json\n" + valid + "\n```",
		strings.Replace(valid, `"commands"`, `"Commands"`, 1),
		strings.Replace(valid, `["go","test","./..."]`, `"go test ./..."`, 1),
		strings.Replace(valid, `"existing Go tests"`, `" "`, 1),
		strings.Replace(valid, `["go.mod"]`, `[]`, 1),
		strings.Replace(valid, `"reason":`, `"status":"passed","reason":`, 1),
		strings.Replace(valid, `"reason":`, `"reason":"duplicate","reason":`, 1),
	} {
		if _, err := ParseDiscovery(invalid); err == nil {
			t.Errorf("accepted %s", invalid)
		}
	}
	for _, input := range []string{valid, `{"commands":[]}`} {
		if _, err := ParseDiscovery(input); err != nil {
			t.Fatal(err)
		}
	}
}
