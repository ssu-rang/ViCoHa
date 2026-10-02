package fixture
import ("testing"; "strings"; "fmt")
func TestOracleNormalize(t *testing.T) {
 inputs := []string{" EXAMPLE ", " ?BC ", "?USER", "", " Mixed Case:ID ", "Already_lower"}
 for i:=0; i<32; i++ { inputs=append(inputs, fmt.Sprintf(" Tenant-%d-ACCOUNT ",i)) }
 for _, input := range inputs { if got,want:=Normalize(input), strings.ToLower(strings.TrimSpace(input)); got!=want { t.Errorf("%q: got %q want %q",input,got,want) } }
}
