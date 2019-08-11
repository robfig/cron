[![GoDoc](http://godoc.org/github.com/robfig/cron?status.png)](http://godoc.org/github.com/robfig/cron) 
[![Build Status](https://travis-ci.org/robfig/cron.svg?branch=master)](https://travis-ci.org/robfig/cron)

# cron

Documentation here: https://godoc.org/github.com/robfig/cron

---

```
添加了 @interval 注解模式，保证每两次定时任务不会重叠执行。
例如:
c := cron.New()
c.AddFunc("@interval 10s", func() {
		time.Sleep(30 * time.Second)
		fmt.Println("interval 10 second to do next")
    })
c.Start()


```

---
```
add @interval feature
Sometimes we want only one job working at the same time, but when the time you cost in once the job is longer than your job interval time, you may have mutilated job working at the same time.

So the @interval schedule can help, each job will be executed at the interval time after the previous job is completed.

e.g.
c := cron.New()
c.AddFunc("@interval 10s", func() {
		time.Sleep(30 * time.Second)
		fmt.Println("interval 10 second to do next")
    })
c.Start()
```