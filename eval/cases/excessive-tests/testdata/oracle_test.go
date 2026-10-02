package fixture
import "testing"
func TestOracleRetry(t *testing.T) { for n:=-1; n<=700; n++ { want:=n==429 || (n>=500 && n<=599); if IsRetryable(n)!=want { t.Errorf("status %d",n) } } }
