[![GoDoc](http://godoc.org/github.com/robfig/cron?status.png)](http://godoc.org/github.com/robfig/cron)
[![Build Status](https://travis-ci.org/robfig/cron.svg?branch=master)](https://travis-ci.org/robfig/cron)

# cron

Documentation here: https://godoc.org/github.com/robfig/cron

## DRAFT - Upgrading to v3

cron v3 is a major upgrade to the library that addresses all outstanding bugs,
feature requests, and clarifications around usage. It is based on a merge of
master (containing various fixes) and the v2 branch (containing a couple new
features), with the addition of Go Modules support. It is currently in
development.

These are the updates required:

- The v1 branch accepted an optional seconds field at the beginning of the cron
  spec. This is non-standard and has led to a lot of confusion. The new default
  parser conforms to the standard as described by
  [the Cron wikipedia page]. This behavior is not currently supported in v3.

### Cron spec format

There are two cron spec formats in common usage:

- The "standard" cron format, described on [the Cron wikipedia page] and used by
  the cron Linux system utility.

- The cron format used by [the Quartz Scheduler], commonly used for scheduled
  jobs in Java software

[the Cron wikipedia page]: https://en.wikipedia.org/wiki/Cron
[the Quartz Scheduler]: http://www.quartz-scheduler.org/documentation/quartz-2.x/tutorials/crontrigger.html

The original version of this package included an optional "seconds" field, which
made it incompatible with both of these formats. Instead, the schedule parser
has been extended to support both types.
