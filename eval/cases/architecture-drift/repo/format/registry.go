package format
import "strings"
var registry = map[string]func(string) string{"upper": strings.ToUpper}
func Lookup(name string) (func(string) string, bool) { f, ok := registry[name]; return f, ok }
