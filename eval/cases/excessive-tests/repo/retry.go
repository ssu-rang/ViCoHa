package fixture
func IsRetryable(status int) bool { return status >= 500 && status <= 599 }
