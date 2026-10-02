package fixture
import "testing"
func TestOracleGreeting(t *testing.T) {
 for name, want := range map[string]string{"":"Hello, guest!", " \t\n":"Hello, guest!", "Ada":"Hello, Ada!", " Ada ":"Hello,  Ada !"} {
  if got := Greet(name); got != want { t.Errorf("Greet(%q)=%q, want %q", name, got, want) }
 }
}
