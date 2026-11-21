package main
import (
  "os"
  "path/filepath"
  "time"
  "github.com/danos/configd/startuplock"
)
func main() {
  base, id := os.Args[1], os.Args[2]
  f,e := startuplock.Acquire(filepath.Join(base,"lock"),10*time.Second); if e!=nil { panic(e) }; defer f.Close()
  if e=os.WriteFile(filepath.Join(base,id+".entered"),[]byte("1"),0600);e!=nil{panic(e)}
  for { if _,e=os.Stat(filepath.Join(base,id+".release"));e==nil{return}; time.Sleep(20*time.Millisecond) }
}
