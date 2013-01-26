////////////////////////////////////////////////////////////////////////////
// Porgram: BailOut
// Purpose: A demo of mechanism that makes sure all clean up and done properly before closing down the program
// Authors: Tong Sun (c) 2013, All rights reserved
////////////////////////////////////////////////////////////////////////////

// Style: gofmt -tabs=false -tabwidth=2
// By Jan Mercl, http://play.golang.org/p/m96skGjRjo

package main

import (
  "log"
  "runtime/debug"
  "time"
)

func deep2(n int) {
  defer func() {
    log.Printf("Resource %d closed", n)
    if err := recover(); err != nil {
      panic(err)
    }
  }()

  for i := 1; i <= 5; i++ {
    log.Println("Working...")
    time.Sleep(time.Second)

  }
  panic("I'm afraid I can't do that")
}

func deep1(n int) {
  defer func() {
    log.Printf("Resource %d closed", n)
    if err := recover(); err != nil {
      panic(err)
    }
  }()
  deep2(n + 1)
}

func main() {
  defer func() {
    log.Print("Main exiting")
    if err := recover(); err != nil {
      log.Fatalf("Stack trace:\n%s----\n%s", debug.Stack(), err)
    }
  }()

  log.Println("Main started")
  deep1(1)
}
