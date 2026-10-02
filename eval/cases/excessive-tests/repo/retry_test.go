package fixture
import "testing"
func TestRetry(t *testing.T) { for _,n:=range []int{500,503,599} { if !IsRetryable(n) { t.Errorf("%d",n) } } }
