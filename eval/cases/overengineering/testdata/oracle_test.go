package fixture
import "testing"
func TestOracleClamp(t *testing.T) {
 for _,c:=range [][4]int{{2,0,5,2},{-2,0,5,0},{8,0,5,5},{-2,-5,-1,-2},{4,3,3,3},{0,0,0,0}} { if got:=Clamp(c[0],c[1],c[2]); got!=c[3] { t.Errorf("Clamp(%v)=%d",c,got) } }
 if Abs(-2)!=2 { t.Fatal("Abs regression") }
}
