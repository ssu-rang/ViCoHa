package fixture
import "testing"
func TestName(t *testing.T) { if Greet("Ada") != "Hello, Ada!" { t.Fatal("greeting changed") } }
