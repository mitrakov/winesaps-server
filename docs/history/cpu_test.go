package main

import "os"
import "log"
import "time"
import "strconv"
import "math/rand"
import "golang.org/x/crypto/bcrypt"

func RandString(n int) string {
    letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, n)
    for i := range b {
        b[i] = letterBytes[rand.Intn(len(letterBytes))]
    }
    return string(b)
}

func main(){
    // parsing arguments
    if len(os.Args) != 3 {
        log.Println("Usage:", os.Args[0], "keys_count goroutines_count\nExample:", os.Args[0], "256 2")
        os.Exit(1)
    }
    n, err1 := strconv.Atoi(os.Args[1])
    m, err2 := strconv.Atoi(os.Args[2])
    if err1 != nil || err2 != nil {
        log.Panicln(err1, err2)
    }
    
    // remember start time
    log.Println("Generating", n, "bcrypt keys on", m, "goroutines")
    t0 := time.Now()
    
    // channel for goroutines
    channel := make(chan bool)
    
    // start goroutines
    for j:=0; j<m; j++ {
        go func() {
            for i:=0; i<n/m; i++ {
                s := RandString(100)
                bcrypt.GenerateFromPassword([]byte(s), 10)
            }
            channel <- true
        }()
    }
    
    // wait for goroutines to stop
    for j:=0; j<m; j++ {
        <- channel
    }
    
    // finish
	log.Println("Elapsed time:", time.Since(t0))
}
