package fixture
import ("io"; "strings")
func FirstLine(r io.Reader) (string,error) { b,err:=io.ReadAll(r); if err!=nil { return "",err }; return strings.SplitN(string(b), "\n", 2)[0],nil }
