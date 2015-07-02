[![GoDoc](http://godoc.org/github.com/robfig/cron?status.png)](http://godoc.org/github.com/robfig/cron)

With some test code:
```go
package main

import (
	"fmt"
	"github.com/elgs/cron"
	"time"
)

func main() {
	c := cron.New()
	fmt.Println("Job 4 status:", c.Status(4))
	j1, _ := c.AddFunc("* * * * * *", func() { fmt.Println("1") })
	j2, _ := c.AddFunc("* * * * * *", func() { fmt.Println("2") })
	j3, _ := c.AddFunc("* * * * * *", func() { fmt.Println("3") })
	j4, _ := c.AddFunc("* * * * * *", func() { fmt.Println("4") })
	fmt.Println(j1, j2, j3, j4)
	c.RemoveFunc(j2)
	fmt.Println("j2 removed")

	c.Start()

	time.Sleep(time.Second * 5)
	c.RemoveFunc(j1)
	fmt.Println("j1 removed")

	fmt.Println("Job 4 status:", c.Status(j4))
	time.Sleep(time.Second * 5)
	c.PauseFunc(j4)
	fmt.Println("j4 paused")
	fmt.Println("Job 4 status:", c.Status(j4))

	time.Sleep(time.Second * 5)
	c.ResumeFunc(j4)
	fmt.Println("j4 resumed")
	fmt.Println("Job 4 status:", c.Status(j4))
	select {}
}

```
And the output of the code above:
```
Job 4 status: -1 // -1 for not started
1 2 3 4
j2 removed
1
3
4
1
3
4
1
3
4
1
3
4
1
3
4
j1 removed
Job 4 status: 0 // 0 for running
3
4
3
4
3
4
3
4
3
4
j4 paused
Job 4 status: 1 // 1 for paused
3
3
3
3
3
j4 resumed
Job 4 status: 0 // 0 for running
3
4
3
4
3
4
```
