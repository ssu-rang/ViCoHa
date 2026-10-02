package fixture
import "testing"
func TestNormalize(t *testing.T) { if Normalize(" customer ") != "customer" { t.Fatal("trim") } }
