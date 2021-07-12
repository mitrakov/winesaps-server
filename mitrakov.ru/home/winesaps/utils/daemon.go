package utils

import "log"
import "time"

// RunDaemon runs a "background service" that calls function "f" periodically with intervals of "t".
// Method returns a channel[Bool], so that the daemon can be stopped with "stopMyDaemon <- true"
// "name" - name of a daemon [optional]
func RunDaemon(name string, t time.Duration, f func()) chan bool {
    log.Println("Starting daemon", name, "with period", t)
    stop := make(chan bool, 1)
    
    go func() {
        ticker := time.NewTicker(t)
        for { // stackoverflow.com/questions/17797754
            select {
                case <- ticker.C: f()
                case <- stop:
                    log.Println("Stopping daemon", name)
                    ticker.Stop()
                    return
            }
        }
    }()
    
    return stop
}

// RunTask starts a "background timer" that will call function "f" after a given duration "t".
// Method returns a channel[Bool], so that the timer can be stopped in advance with "stopMyTimer <- true"
// "name" - name of a timer [optional]
func RunTask(name string, t time.Duration, f func()) chan bool {
    log.Println("Starting task", name, "with timeout", t)
    stop := make(chan bool, 1)
    
    go func() {
        timer := time.NewTimer(t)
        for { // stackoverflow.com/questions/17797754
            select {
                case <- timer.C: 
                    log.Println("Task", name, "timed out")
                    f()
                    return
                case <- stop:
                    log.Println("Stopping task", name)
                    timer.Stop()
                    return
            }
        }
    }()
    
    return stop
}
