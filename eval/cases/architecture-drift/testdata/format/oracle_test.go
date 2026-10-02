package format
import "testing"
func TestOracleRegistry(t *testing.T) {
 f,ok:=Lookup("lower"); if !ok { t.Fatal("lower missing from registry") }; if f("?BC")!="?bc" { t.Fatal("Unicode lower") }
 if f,ok=Lookup("upper"); !ok || f("Abc")!="ABC" { t.Fatal("upper regression") }
 if _,ok=Lookup("unknown"); ok { t.Fatal("unknown format") }
 if _,ok:=registry["lower"]; !ok { t.Fatal("lower must extend existing registry") }
}
