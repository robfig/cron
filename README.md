# go-cron

[![GoDoc](http://godoc.org/github.com/lestrrat/cron?status.png)](http://godoc.org/github.com/lestrrat/cron)

This is a fork of [github.com/robfig/cron](https://github.com/robfig/cron).
Following are the differences from the original version As of this writing:

* Use context.Context to control dispatcher
* Hide structs where applicable, and use interfaces instead
* Fix some synchronization issues
* Added more tests
* Removed panics
