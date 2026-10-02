package fixture
import ("testing"; "strings"; "io"; "errors")
type counted struct { r io.Reader; n int }
func (c *counted) Read(p []byte)(int,error) { n,e:=c.r.Read(p); c.n+=n; return n,e }
type broken struct{}
func (broken) Read([]byte)(int,error) { return 0,io.ErrUnexpectedEOF }
func TestOracleFirstLine(t *testing.T) {
 r:=&counted{r:io.MultiReader(strings.NewReader("header\n"),strings.NewReader(strings.Repeat("x",1<<20)))}
 if got,e:=FirstLine(r); e!=nil || got!="header" { t.Fatalf("%q %v",got,e) }; if r.n>8192 { t.Fatalf("read %d bytes for short header",r.n) }
 for _,s:=range []string{"", "tail", " spaced ", strings.Repeat("y",100000)} { if got,e:=FirstLine(strings.NewReader(s)); e!=nil || got!=s { t.Fatal("EOF handling",e) } }
 if _,e:=FirstLine(broken{}); !errors.Is(e,io.ErrUnexpectedEOF) { t.Fatal("lost read error",e) }
}
